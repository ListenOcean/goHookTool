package instrument

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ListenOcean/goHookTool/configs"

	"github.com/dave/dst"
)

type mainPackageInstrumentation struct {
	*defaultPackageInstrumentation
}

func NewMainPackageInstrumentation(pkgPath string, fullInstrumentation bool, packageBuildDir string) *mainPackageInstrumentation {
	return &mainPackageInstrumentation{
		defaultPackageInstrumentation: NewDefaultPackageInstrumentation(pkgPath, fullInstrumentation, packageBuildDir),
	}
}

func (m *mainPackageInstrumentation) IsIgnored() bool {
	return false
}

func (m *mainPackageInstrumentation) Instrument() ([]*dst.File, error) {
	if m.defaultPackageInstrumentation.IsIgnored() {
		return nil, nil
	}
	return m.defaultPackageInstrumentation.Instrument()
}

func (m *mainPackageInstrumentation) WriteExtraFiles() (extra []string, err error) {
	if !m.defaultPackageInstrumentation.IsIgnored() {
		extra, err = m.defaultPackageInstrumentation.WriteExtraFiles()
		if err != nil {
			return nil, err
		}
	}

	if ht, err := m.writeHookTable(); err != nil {
		return nil, err
	} else if ht != "" {
		extra = append(extra, ht)
	}

	return extra, nil
}

func removeBracketsAndStar(s string) string {
	if strings.HasPrefix(s, "(") {
		s = s[1 : len(s)-1]
		if strings.HasPrefix(s, "*") {
			s = s[1:]
			return s
		}
		return s
	}
	return s
}

func normalizedSignatrue(pkgpath, sign string) string {
	normalizedPkgPath := regexp.MustCompile(`[/.\-@]`).ReplaceAllString(pkgpath, "_")
	sign = strings.TrimPrefix(sign, pkgpath+".")
	signSlice := strings.Split(sign, ".")
	nameSlice := []string{}
	for _, eachStr := range signSlice {
		nameSlice = append(nameSlice, removeBracketsAndStar(eachStr))
	}
	return fmt.Sprintf("%s_%s", normalizedPkgPath, strings.Join(nameSlice, "_"))

}

func (m *mainPackageInstrumentation) writeHookTable() (string, error) {
	// Create the hook table and compile it.
	// Get the full list of hooks
	hooks, err := readHookListFile(m.hookListFilepath)
	if err != nil {
		return "", err
	}

	hookPointSet := make(map[string]struct{})
	for _, hookpoint := range hooks {
		hp := strings.TrimPrefix(hookpoint, configs.HookDescriptorIdentPrefix)
		hookPointSet[hp] = struct{}{}
	}
	log.Printf("Not Hooked:\n")

	countConfigHookPoint := 0
	for pkgname, mapdata := range configs.HookPointMap {
		for signatrue := range mapdata {
			countConfigHookPoint += 1
			hookpoint := normalizedSignatrue(pkgname, signatrue)
			if _, ok := hookPointSet[hookpoint]; !ok {
				log.Printf("%s\n", hookpoint)
			}
		}
	}
	log.Printf("Loaded %d HookPoints in configs.", countConfigHookPoint)
	log.Printf("Hooked %d HookPoints in hooktable.", len(hooks))

	if len(hooks) == 0 {
		log.Printf("skipping hook table generation: the list of hooks is empty")
		return "", nil
	}

	// Create the hook table file into the package build directory
	hookTableFile, err := createHookTableFile(m.packageBuildDir)
	if err != nil {
		return "", err
	}
	defer hookTableFile.Close()
	log.Printf("creating the hook table for %d hooks from `%s` into `%s`", len(hooks), m.hookListFilepath, hookTableFile.Name())
	if err := writeHookTable(hookTableFile, hooks); err != nil {
		return "", err
	}

	// Add it into the argument list to compile it
	return hookTableFile.Name(), nil
}

func createHookTableFile(dir string) (*os.File, error) {
	filename := filepath.Join(dir, "hooktable.go")
	return os.OpenFile(filename, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
}

// Create or append the hook list file in write-only.
func openHookListFile(hookListFilepath string) (*os.File, error) {
	return os.OpenFile(hookListFilepath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
}

func getHookListFilepath(dir string) string {
	return filepath.Join(dir, "hooks.txt")
}

// Read the given hook list file by reopening it and reading its full content,
// returned as a slice of hook IDs.
func readHookListFile(hookListFilepath string) (hooks []string, err error) {
	f, err := os.OpenFile(hookListFilepath, os.O_RDONLY, 0666)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	// Read each hook id line by line
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		id := scanner.Text()
		hooks = append(hooks, id)
	}
	return
}
