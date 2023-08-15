package instrument

import (
	"github.com/ListenOcean/goHookTool/internal/toolexec/ast"
	"github.com/ListenOcean/goHookTool/utils"

	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
)

type runtimeInstrumentationVisitor struct {
	packageInstrumentationHelper
	defaultVisitor *defaultPackageInstrumentationVisitor
}

func newRuntimeInstrumentationVisitor(pkgPath string, instrumentedFiles map[*dst.File][]*ast.Hookpoint) *runtimeInstrumentationVisitor {
	utils.NotNil(instrumentedFiles)

	hookDescriptorTypeDecl, hookDescriptorTypeSpec, newDescriptorValueInitializer := ast.NewHookDescriptorType()
	hookDescriptorTypeIdent := hookDescriptorTypeSpec.Name.Name
	return &runtimeInstrumentationVisitor{
		defaultVisitor: &defaultPackageInstrumentationVisitor{
			pkgPath:                           pkgPath,
			instrumentedHooks:                 instrumentedFiles,
			hookDescriptorTypeIdent:           hookDescriptorTypeIdent,
			hookDescriptorTypeDecl:            hookDescriptorTypeDecl,
			newHookDescriptorValueInitializer: newDescriptorValueInitializer,
		},
	}
}

func (v *runtimeInstrumentationVisitor) instrument(root *dst.Package) []*dst.File {
	dstutil.Apply(root, v.defaultVisitor.instrumentPre, v.defaultVisitor.instrumentPost)

	instrumentedFileSet := make(map[*dst.File]struct{})
	for _, eachFile := range v.defaultVisitor.instrumentedFiles {
		instrumentedFileSet[eachFile] = struct{}{}
	}

	instrumentedFiles := []*dst.File{}
	for eachFile := range instrumentedFileSet {
		instrumentedFiles = append(instrumentedFiles, eachFile)
	}
	return instrumentedFiles
}
