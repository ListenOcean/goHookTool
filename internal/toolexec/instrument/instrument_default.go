package instrument

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/ListenOcean/goHookTool/configs"
	"github.com/ListenOcean/goHookTool/internal/toolexec/ast"
	"github.com/ListenOcean/goHookTool/utils"

	"github.com/dave/dst"
)

type defaultPackageInstrumentation struct {
	packageInstrumentationHelper
	instrumentedFiles   map[*dst.File][]*ast.Hookpoint
	fullInstrumentation bool
	hookListFilepath    string
	packageBuildDir     string
}

func NewDefaultPackageInstrumentation(pkgPath string, fullInstrumentation bool, packageBuildDir string) *defaultPackageInstrumentation {
	projectBuildDir := filepath.Join(packageBuildDir, "..")
	hookListFilepath := getHookListFilepath(projectBuildDir)

	return &defaultPackageInstrumentation{
		packageInstrumentationHelper: makePackageInstrumentationHelper(pkgPath),
		fullInstrumentation:          fullInstrumentation,
		hookListFilepath:             hookListFilepath,
		packageBuildDir:              packageBuildDir,
	}
}

func (h *defaultPackageInstrumentation) IsIgnored() bool {
	// Check if the instrumentation should be skipped for this package name.
	return h.isPackageIgnored()
}

func (h *defaultPackageInstrumentation) isPackageIgnored() bool {
	for _, prefix := range configs.IgnoredPkgPrefixes {
		if strings.HasPrefix(h.pkgPath, prefix) {
			return true
		}
	}

	if h.fullInstrumentation {
		return false
	}

	// 当前包名在HookPointMap中，说明需要被插桩
	if _, ok := configs.HookPointMap[h.pkgPath]; ok {
		return false
	}

	return true
}

func UnvendorPackagePath(pkg string) (unvendored string) {
	return utils.Unvendor(pkg)
}

func (h *defaultPackageInstrumentation) Instrument() (instrumented []*dst.File, err error) {
	h.instrumentedFiles = make(map[*dst.File][]*ast.Hookpoint)
	v := newDefaultPackageInstrumentationVisitor(h.pkgPath, h.instrumentedFiles)
	return h.packageInstrumentationHelper.instrument(v)
}

func (h *defaultPackageInstrumentation) writeHookList(hookList *os.File) (count int, err error) {
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

func (h *defaultPackageInstrumentation) WriteExtraFiles() (extra []string, err error) {
	// Add the hook IDs to the hook list file.
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

	return extra, nil
}
