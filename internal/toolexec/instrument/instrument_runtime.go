package instrument

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/ListenOcean/goHookTool/configs"
	"github.com/ListenOcean/goHookTool/internal/toolexec/ast"

	"github.com/dave/dst"
)

type runtimePackageInstrumentation struct {
	packageInstrumentationHelper
	instrumentedFiles   map[*dst.File][]*ast.Hookpoint
	fullInstrumentation bool
	hookListFilepath    string
	packageBuildDir     string
}

func NewRuntimePackageInstrumentation(pkgPath string, fullInstrumentation bool, packageBuildDir string) *runtimePackageInstrumentation {
	projectBuildDir := filepath.Join(packageBuildDir, "..")
	hookListFilepath := getHookListFilepath(projectBuildDir)
	return &runtimePackageInstrumentation{
		packageInstrumentationHelper: makePackageInstrumentationHelper(pkgPath),
		fullInstrumentation:          fullInstrumentation,
		hookListFilepath:             hookListFilepath,
		packageBuildDir:              packageBuildDir,
	}
}

func (runtimePackageInstrumentation) IsIgnored() bool {
	// This instrumentation is never ignored
	return false
}

func (h *runtimePackageInstrumentation) Instrument() (instrumented []*dst.File, err error) {
	h.instrumentedFiles = make(map[*dst.File][]*ast.Hookpoint)
	v := newRuntimeInstrumentationVisitor(h.pkgPath, h.instrumentedFiles)
	return h.packageInstrumentationHelper.instrument(v)
}

func (h *runtimePackageInstrumentation) writeHookList(hookList *os.File) (count int, err error) {
	for _, hooks := range h.instrumentedFiles {
		for _, hook := range hooks {
			if _, err = hookList.WriteString(fmt.Sprintf("%s\n", hook.DescriptorFuncDecl.Name.Name)); err != nil {
				return count, err
			}
			count += 1
		}
	}
	return count, nil
}

func (h *runtimePackageInstrumentation) WriteExtraFiles() ([]string, error) {
	// 写入hook列表
	hookListFile, err := openHookListFile(h.hookListFilepath)
	if err != nil {
		return nil, err
	}
	defer hookListFile.Close()
	count, err := h.writeHookList(hookListFile)
	if err != nil {
		return nil, err
	}
	log.Printf("added %d hooks to the hook list\n", count)

	rtExtensions := filepath.Join(h.packageBuildDir, "autobuild.go")
	if err := os.WriteFile(rtExtensions, []byte(configs.RuntimeExtraFileContent), 0644); err != nil {
		return nil, err
	}
	return []string{rtExtensions}, nil
}
