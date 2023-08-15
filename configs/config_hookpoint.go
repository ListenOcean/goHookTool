package configs

type Config struct {
	Hookpoints map[string][]string `yaml:"hookpoints"`
	Codes      map[string]Code     `yaml:"codes"`
}

type Code struct {
	Epilog string `yaml:"epilog"`
	Prolog string `yaml:"prolog"`
}

var ConfigData Config

// Hook点，pkgname =>set of signatrue
var HookPointMap = map[string]map[string]struct{}{}

// default构建器时（不等于main、runtime的包）会忽略的前缀
var IgnoredPkgPrefixes = []string{
	"runtime", // 用于忽略runtime里面的所有包，例如runtime/internal/xxx也会进入default构建器逻辑
	"sync",
	"reflect",
	"internal",
	"unsafe",
	"syscall",
	"time",
}
