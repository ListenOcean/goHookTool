package instrument

import (
	"fmt"
	"go/parser"
	"go/token"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/ListenOcean/goHookTool/internal/toolexec/ast"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
)

type Instrumenter interface {
	IsIgnored() bool
	AddFile(src string) error
	Instrument() ([]*dst.File, error)
	WriteInstrumentedFiles(packageBuildDir string, instrumented []*dst.File) (srcdst map[string]string, err error)
	WriteExtraFiles() ([]string, error)
}

type packageInstrumentationHelper struct {
	parsedFiles       map[string]*dst.File
	parsedFileSources map[*dst.File]string
	fset              *token.FileSet
	pkgPath           string
}

func makePackageInstrumentationHelper(pkgPath string) packageInstrumentationHelper {
	// Remove the package path vendor prefix so that everything, from this tool to
	// the agent instrumentation package works properly with the package path names
	// as if it wasn't vendored. By doing so, things like checking if the package
	// should be ignored, or looking up a hook descriptor is simplified and can
	// completely ignore the vendoring.
	pkgPath = UnvendorPackagePath(pkgPath)

	return packageInstrumentationHelper{
		pkgPath: pkgPath,
	}
}

// AddFile parses the given Go source file `src` and adds it to the set of
// files to instrument if it is not ignored by a directive.
func (h *packageInstrumentationHelper) AddFile(src string) error {
	// Check if the instrumentation should be skipped for this filename
	if isFileNameIgnored(src) {
		log.Println("skipping instrumentation of file", src)
		return nil
	}

	log.Printf("parsing file `%s`", src)
	if h.fset != nil {
		// The token fileset is required to later create the package node.
		h.fset = token.NewFileSet()
	}
	file, err := decorator.ParseFile(h.fset, src, nil, parser.ParseComments)
	if err != nil {
		return err
	}

	// Check if there is a file-level ignore directive
	if ast.HasIgnoreDirective(file) {
		log.Printf("file `%s` skipped due to ignore directive", src)
		return nil
	}
	if h.parsedFiles == nil {
		h.parsedFiles = make(map[string]*dst.File)
		h.parsedFileSources = make(map[*dst.File]string)
	}
	h.parsedFiles[src] = file
	h.parsedFileSources[file] = src
	return nil
}

func isFileNameIgnored(file string) bool {
	filename := filepath.Base(file)
	// Don't instrument cgo files
	if strings.Contains(filename, "cgo") {
		return true
	}
	// Don't instrument the go module table file.
	if filename == "_gomod_.go" {
		return true
	}
	return false
}

func (h *packageInstrumentationHelper) instrument(v instrumentationVisitorFace) (instrumented []*dst.File, err error) {
	if len(h.parsedFiles) == 0 {
		log.Println("nothing to instrument")
		return nil, nil
	}

	root, err := dst.NewPackage(h.fset, h.parsedFiles, nil, nil)
	if err != nil {
		return nil, err
	}

	return v.instrument(root), nil
}

func (h *packageInstrumentationHelper) WriteInstrumentedFiles(buildDirPath string, instrumentedFiles []*dst.File) (srcdst map[string]string, err error) {
	srcdst = make(map[string]string, len(instrumentedFiles))
	for _, node := range instrumentedFiles {
		src := h.parsedFileSources[node]
		filename := filepath.Base(src)
		dest := filepath.Join(buildDirPath, filename)
		output, err := os.Create(dest)
		if err != nil {
			return nil, err
		}
		defer output.Close()
		// Add a go line directive in order to map it to its original source file.
		// Note that otherwise it uses the build directory but it is trimmed by the
		// compiler - so you end up with filenames without any leading path (eg.
		// myfile.go) leading to broken debuggers or stack traces.
		// 添加go line编译指令`//line <filename>:1`，使得编译器构建时能够将其映射到原始文件，而不使用临时目录中的文件
		output.WriteString(fmt.Sprintf("//line %s:1\n", src))
		if err := ast.WriteFile(node, output); err != nil {
			return nil, err
		}
		srcdst[src] = dest
	}
	return srcdst, nil
}

//func getToken(length int) string {
//	randomBytes := make([]byte, 32)
//	_, err := rand.Read(randomBytes)
//	if err != nil {
//		panic(err)
//	}
//	return base32.StdEncoding.EncodeToString(randomBytes)[:length]
//}

// Debug用途，将修改后的文件随机命名，配合脚本转移到debug目录，使用dlv时可以索引到源码

//func (h *packageInstrumentationHelper) WriteInstrumentedFiles(buildDirPath string, instrumentedFiles []*dst.File) (srcdst map[string]string, err error) {
//	srcdst = make(map[string]string, len(instrumentedFiles))
//	for _, node := range instrumentedFiles {
//		src := h.parsedFileSources[node]
//		//filename := filepath.Base(src)
//		//dest := filepath.Join(buildDirPath, filename)
//		dest := filepath.Join(buildDirPath, getToken(20)+".go")
//		output, err := os.Create(dest)
//		if err != nil {
//			return nil, err
//		}
//		defer output.Close()
//		// Add a go line directive in order to map it to its original source file.
//		// Note that otherwise it uses the build directory but it is trimmed by the
//		// compiler - so you end up with filenames without any leading path (eg.
//		// myfile.go) leading to broken debuggers or stack traces.
//		// 添加go line编译指令`//line <filename>:1`，使得编译器构建时能够将其映射到原始文件，而不使用临时目录中的文件
//		//output.WriteString(fmt.Sprintf("//line %s:1\n", src))
//		if err := ast.WriteFile(node, output); err != nil {
//			return nil, err
//		}
//		srcdst[src] = dest
//	}
//	return srcdst, nil
//}
