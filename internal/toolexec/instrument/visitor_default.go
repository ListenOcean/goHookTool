package instrument

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"sort"
	"strings"

	"github.com/ListenOcean/goHookTool/configs"
	"github.com/ListenOcean/goHookTool/internal/toolexec/ast"
	"github.com/ListenOcean/goHookTool/utils"

	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
)

type instrumentationVisitorFace interface {
	instrument(root *dst.Package) (instrumented []*dst.File)
}

type defaultPackageInstrumentationVisitor struct {
	// Instrumentation statistics of the currently instrumented package.
	stats instrumentationStats
	// Package path being instrumented. Used to generate unique hook names
	// prefixed by the package path.
	pkgPath string
	// False when the first file is being instrumented in order to add
	// metadata that must appear once.
	fileMetadataOnce bool
	// List of hookpoints in the current file being instrumented.
	instrumented []*ast.Hookpoint
	// Map of instrumented files along with there hookpoints
	instrumentedHooks map[*dst.File][]*ast.Hookpoint
	// Slice of instrumented files
	instrumentedFiles []*dst.File
	// Hook descriptor type declaration node. It will be added to the file
	// metadata.
	hookDescriptorTypeIdent string
	// The hook descriptor value initializer used by the hook descriptor function
	// in order to create a new descriptor value.
	newHookDescriptorValueInitializer ast.HookDescriptorValueInitializer
	// The hook descriptor type declaration added once per instrumented package
	// and used by hook descriptor functions to return a value of that type.
	hookDescriptorTypeDecl *dst.GenDecl
}

type instrumentationStats struct {
	ignored []string
}

func (s *instrumentationStats) addIgnored(funcDecl *dst.FuncDecl) {
	s.ignored = append(s.ignored, funcDecl.Name.Name)
}

func newDefaultPackageInstrumentationVisitor(pkgPath string, instrumentedFiles map[*dst.File][]*ast.Hookpoint) *defaultPackageInstrumentationVisitor {
	utils.NotNil(instrumentedFiles)

	hookDescriptorTypeDecl, hookDescriptorTypeSpec, newDescriptorValueInitializer := ast.NewHookDescriptorType()
	hookDescriptorTypeIdent := hookDescriptorTypeSpec.Name.Name
	return &defaultPackageInstrumentationVisitor{
		pkgPath:                           pkgPath,
		instrumentedHooks:                 instrumentedFiles,
		hookDescriptorTypeIdent:           hookDescriptorTypeIdent,
		hookDescriptorTypeDecl:            hookDescriptorTypeDecl,
		newHookDescriptorValueInitializer: newDescriptorValueInitializer,
	}
}

func (v *defaultPackageInstrumentationVisitor) makeSignatrue(funcDecl *dst.FuncDecl) string {
	var res strings.Builder
	res.WriteString(v.pkgPath)
	if funcDecl.Recv != nil {
		utils.True(len(funcDecl.Recv.List) == 1)
		switch funcDecl.Recv.List[0].Type.(type) {
		case *dst.StarExpr:
			res.WriteString(".(*")
			res.WriteString(funcDecl.Recv.List[0].Type.(*dst.StarExpr).X.(*dst.Ident).Name)
			res.WriteString(")")
		case *dst.Ident:
			res.WriteString(".")
			res.WriteString(funcDecl.Recv.List[0].Type.(*dst.Ident).Name)
		}
	}
	res.WriteString(".")
	res.WriteString(funcDecl.Name.Name)
	return res.String()
}

func (v *defaultPackageInstrumentationVisitor) instrumentFuncDeclPre(funcDecl *dst.FuncDecl) {
	signatrue := v.makeSignatrue(funcDecl)
	if ast.ShouldIgnoreFuncDecl(funcDecl) {
		v.stats.addIgnored(funcDecl)
		return
	}

	if signatrueSet, ok := configs.HookPointMap[v.pkgPath]; ok {
		if _, ok := signatrueSet[signatrue]; ok {
			log.Printf("Will hook: %s\n", signatrue)
			hook := ast.NewHookpoint(signatrue, v.pkgPath, funcDecl, v.hookDescriptorTypeIdent, v.newHookDescriptorValueInitializer)
			v.instrumented = append(v.instrumented, hook)
			funcDecl.Body.List = append([]dst.Stmt{hook.InstrumentationStmt}, funcDecl.Body.List...)
		}
	}
}

func (v *defaultPackageInstrumentationVisitor) instrument(root *dst.Package) (instrumented []*dst.File) {
	dstutil.Apply(root, v.instrumentPre, v.instrumentPost)
	return v.instrumentedFiles
}

func (v *defaultPackageInstrumentationVisitor) instrumentPre(cursor *dstutil.Cursor) bool {
	switch node := cursor.Node().(type) {
	case *dst.FuncDecl:
		v.instrumentFuncDeclPre(node)
		// Note that we don't add the file metadata here in order to avoid to
		// infinite traversal because of adding new AST nodes while visiting it.

		// No need to go deeper than function declarations
		return false
	}
	return true
}

func (v *defaultPackageInstrumentationVisitor) instrumentPost(cursor *dstutil.Cursor) bool {
	switch node := cursor.Node().(type) {
	case *dst.File:
		v.instrumentFilePost(node)
	}
	return true
}

func (v *defaultPackageInstrumentationVisitor) instrumentFilePost(file *dst.File) {
	if len(v.instrumented) == 0 {
		// Nothing got instrumented
		return
	}
	// Add file-level instrumentation metadata
	v.addFileMetadata(file, v.instrumented)

	// Add the list of hooks of this file node
	v.instrumentedHooks[file] = v.instrumented
	v.instrumented = nil

	// Add the file node in the list of instrumented files
	v.instrumentedFiles = append(v.instrumentedFiles, file)
}

func (v *defaultPackageInstrumentationVisitor) addFileMetadata(file *dst.File, instrumented []*ast.Hookpoint) {
	ast.AddUnsafePackageImport(file)
	// runtime包不需要再次引入_atomic_load_pointer的定义（addAtomicLoadFuncDecl）
	if v.pkgPath == "runtime" {
		v.fileMetadataOnce = true
		v.addHookDescriptorType(file)
	}
	if !v.fileMetadataOnce {
		v.fileMetadataOnce = true
		v.addAtomicLoadFuncDecl(file)
		v.addHookDescriptorType(file)
	}
	for _, h := range instrumented {
		v.addHookMetadata(file, h)
	}
}

func (v *defaultPackageInstrumentationVisitor) addHookDescriptorFuncDecl(file *dst.File, h *ast.Hookpoint) {
	file.Decls = append(file.Decls, h.DescriptorFuncDecl)
}

func (v *defaultPackageInstrumentationVisitor) addHookPrologLoadFuncDecl(file *dst.File, h *ast.Hookpoint) {
	file.Decls = append(file.Decls, h.PrologLoadFuncDecl)
}

func (v *defaultPackageInstrumentationVisitor) addHookMetadata(file *dst.File, h *ast.Hookpoint) {
	v.addHookPrologVarDecl(file, h)
	v.addHookPrologLoadFuncDecl(file, h)
	v.addHookDescriptorFuncDecl(file, h)
}

func (v *defaultPackageInstrumentationVisitor) addHookPrologVarDecl(file *dst.File, h *ast.Hookpoint) {
	file.Decls = append(file.Decls, h.PrologVarDecl)
}

func (v *defaultPackageInstrumentationVisitor) addAtomicLoadFuncDecl(file *dst.File) {
	file.Decls = append(file.Decls, ast.NewLinkTimeAtomicLoadPointerFuncDecl())
}

func (v *defaultPackageInstrumentationVisitor) addHookDescriptorType(file *dst.File) {
	file.Decls = append(file.Decls, v.hookDescriptorTypeDecl)
}

// Write into `w` the Go sources of the hook table for the list of hook
// descriptor function `hooks`.
func writeHookTable(w io.Writer, hooks []string) error {
	sort.Strings(hooks)

	// In case the hook descriptor type hasn't been created, we recreate the
	// type alias again in the hook table file and with a distinct name.
	const (
		tableFormat = `var _hook_table_array = []func(*_hook_table_hook_descriptor_type){%s
}

type _hook_table_type = []func(*_hook_table_hook_descriptor_type)
type _instrumentation_descriptor_type = struct {
	Version   string
	HookTable _hook_table_type
}

//go:linkname _instrumentation_descriptor _instrumentation_descriptor
var _instrumentation_descriptor = &_instrumentation_descriptor_type{
	Version: %q,
	HookTable: _hook_table_array,
}
`
		tableInitListEntryFormat = "\n\t%s,"

		hookDescriptorForwardFuncDeclFormat = `//go:linkname %[1]s %[1]s
func %[1]s(*_hook_table_hook_descriptor_type)

`
		fileFormat = `package main

import _ "unsafe"

type _hook_table_hook_descriptor_type = struct {	Func, Prolog interface{} }

%s

%s
`
	)

	var tableInitList, hookDescriptorForwardFuncDecls bytes.Buffer

	for _, hookDescriptorFuncName := range hooks {
		// Create the hook table initializer entry line
		tableInitListEntry := fmt.Sprintf(tableInitListEntryFormat, hookDescriptorFuncName)
		if _, err := io.WriteString(&tableInitList, tableInitListEntry); err != nil {
			return err
		}

		// We are writing a file for the main package so we don't need to forward
		// declare the hook descriptor functions that are defined in the main
		// package.
		if ast.IsHookDescriptorFuncInMainPackage(hookDescriptorFuncName) {
			continue
		}

		// Create forward declaration of the hook descriptor function
		hookDescriptorForwardFuncDecl := fmt.Sprintf(hookDescriptorForwardFuncDeclFormat, hookDescriptorFuncName)
		if _, err := io.WriteString(&hookDescriptorForwardFuncDecls, hookDescriptorForwardFuncDecl); err != nil {
			return err
		}
	}

	hookTableVar := fmt.Sprintf(tableFormat, &tableInitList, configs.Version)
	_, err := io.WriteString(w, fmt.Sprintf(fileFormat, &hookDescriptorForwardFuncDecls, hookTableVar))
	return err
}
