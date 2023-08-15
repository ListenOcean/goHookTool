package configs

const (
	UnsafePackageName          = `_unsafe_`
	IgnoreDirective            = `//autobuild:ignore`
	HookDescriptorParamName    = `_hd`
	AtomicLoadPointerFuncIdent = `_atomic_load_pointer`
	PrologLoadFuncIdentFormat  = `_hook_prolog_load_%s`

	HookDescriptorIdentPrefix                  = `_hook_descriptor_`
	HookDescriptorTypeIdent                    = HookDescriptorIdentPrefix + `type`
	HookDescriptorFuncIdentFormat              = HookDescriptorIdentPrefix + `%s`
	HookDescriptorFuncIdentPrefixOfMainPackage = HookDescriptorIdentPrefix + `main_`

	PrologVarIdentPrefix = `_hook_prolog_var_`
	PrologVarIdentFormat = PrologVarIdentPrefix + `%s`

	PrologVarIdent           = "_prolog"
	PrologAbortErrorVarIdent = "_prolog_abort_err"
	EpilogVarIdent           = "_epilog"
	NilIdent                 = "nil"

	Version = "0.0.1"
)

var RuntimeExtraFileContent = `package runtime

import (
	"runtime/internal/atomic"
	"unsafe" // also required for go:linkname
)

// 指针读取

//go:linkname _atomic_load_pointer _atomic_load_pointer
//go:nosplit
func _atomic_load_pointer(addr unsafe.Pointer) unsafe.Pointer {
	return atomic.Loadp(addr)
}
`

const CodeTemplate = `package a

func main(){
	%s
}
`
