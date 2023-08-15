package hooklib

import (
	"fmt"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync/atomic"
	"unsafe"

	"github.com/ListenOcean/goHookTool/utils"
	"github.com/pkg/errors"
)

// Types to sync with the instrumentation tool
type (
	InstrumentationDescriptorType = struct {
		Version   string
		HookTable HookTableType
	}
	HookTableType          = []HookDescriptorFuncType
	HookDescriptorFuncType = func(*HookDescriptorType)
	HookDescriptorType     = struct {
		Func, PrologVar interface{}
	}
)

//go:linkname _instrumentation_descriptor _instrumentation_descriptor
var _instrumentation_descriptor *InstrumentationDescriptorType

type symbolIndexType map[string]*Hook

// index of hooks by symbol string. The index is lazily created when symbols
// are searched. Note that due to the large amount of hooks, we avoid having
// a map of hook pointer in order to avoid GC overhead.
var index = make(symbolIndexType)

type Hook struct {
	// Symbol name of the function the hook is associated with.
	symbol string
	// Prolog function type expected by this hook.
	prologFuncType reflect.Type
	// Pointer to the prolog pointer. The value has type **prologFuncType, which
	// is checked at hook creation.
	prologVarAddr *unsafe.Pointer
}

func (h *Hook) GetPrologFuncType() reflect.Type {
	return h.prologFuncType
}

func (h *Hook) String() string {
	return fmt.Sprintf("%s (%s)", h.symbol, h.prologFuncType)
}

// PrologCallback is an interface to a prolog function.
// Given a function F:
//
//	func F(A, B, C) (R, S, T)
//
// The expected prolog signature is:
//
//	type prolog = func(*A, *B, *C) (epilog, error)
//
// The expected epilog signature is:
//
//	type epilog = func(*R, *S, *T)
//
// The returned epilog value can be nil when there is no need for epilog.
type (
	PrologCallback       interface{}
	PrologCallbackGetter interface {
		PrologCallback() PrologCallback
	}
	ReflectedPrologCallback = func(params []reflect.Value) (epilog ReflectedEpilogCallback, err error)
	ReflectedEpilogCallback = func(results []reflect.Value)
)

// Errors that hooks can return in order to modify the control flow of the
// function.
type Error int

const (
	_ Error = iota
	// Abort the execution of the function by returning from it.
	AbortError
)

func (e Error) Error() string {
	switch e {
	case AbortError:
		return "abort function execution"
	default:
		return "unknown"
	}
}

// Static assertion that `Error` implements interface `error`
var _ error = Error(0)

func Health(expectedVersion string) error {
	if _instrumentation_descriptor == nil || len(_instrumentation_descriptor.HookTable) == 0 {
		return errors.New("the program is not instrumented")
	}

	if version := _instrumentation_descriptor.Version; version != expectedVersion {
		return errors.Errorf("the program is not properly instrumented: the agent and instrumentation tool versions must be the same - the tool version is `%s` while the agent version is `%s`", version, expectedVersion)
	}

	return nil
}

// Find returns the hook associated to the given symbol string when it was
// created using `New()`, nil otherwise.
func Find(symbol string) (*Hook, error) {
	return index.find(symbol)
}

// Try to find the `symbol` in the index first, otherwise try to load it from
// the hook table.
func (t symbolIndexType) find(symbol string) (*Hook, error) {
	// Lookup the symbol index first
	if hook, exists := index[symbol]; exists {
		return hook, nil
	}
	if _instrumentation_descriptor != nil {
		// Not found in the index: lookup the hook table
		return hookTableLookup(_instrumentation_descriptor.HookTable, symbol, index)
	} else {
		return nil, fmt.Errorf("_instrumentation_descriptor is empty")
	}
}

func hookTableLookup(table HookTableType, symbol string, index symbolIndexType) (found *Hook, err error) {
	id := normalizedHookID(symbol)
	// The API of sort.Search doesn't allow to abort, so we panic instead,
	// caught by sqsafe.Call.
	sort.Search(len(table), func(i int) bool {
		entry := table[i]
		var descriptor HookDescriptorType
		entry(&descriptor)
		hook, err := index.add(descriptor.Func, descriptor.PrologVar)
		if err != nil {
			panic(err) // abort
		}
		current := normalizedHookID(hook.symbol)
		cmp := strings.Compare(current, id)
		if cmp == 0 {
			found = hook
		}
		return cmp >= 0
	})
	if err != nil {
		return nil, fmt.Errorf("hook table lookup of symbol `%s`,err:%s", symbol, err)
	}
	return found, nil
}

func normalizedHookID(symbol string) string {
	id := regexp.MustCompile(`[ *()]`).ReplaceAllString(symbol, "")
	return regexp.MustCompile(`[/.\-@]`).ReplaceAllString(id, "_")
}

// add creates the hook object for function `fn`, adds it to the find map and
// returns it. It returns an error if it is not possible.
func (t symbolIndexType) add(fn, prologVar interface{}) (h *Hook, err error) {
	// Check fn is a non-nil function value
	if fn == nil {
		return nil, errors.New("unexpected function argument value `nil`")
	}
	fnValue := reflect.ValueOf(fn)
	fnType := fnValue.Type()
	if fnType.Kind() != reflect.Func {
		return nil, errors.Errorf("unexpected function argument type: expecting a function value but got `%T`", fn)
	}

	// Get the symbol name
	symbol := runtime.FuncForPC(fnValue.Pointer()).Name()
	if symbol == "" {
		return nil, errors.Errorf("could not read the symbol name of function `%T`", fn)
	}

	// Unvendor it so that it is not prefixed by `<app>/vendor/`
	symbol = utils.Unvendor(symbol)

	// Use the symbol name for better error messages
	defer func() {
		if err != nil {
			err = errors.Wrapf(err, "symbol `%s`", symbol)
		}
	}()

	// The hook may have been already added by a previous lookup
	if hook, exists := t[symbol]; exists {
		return hook, nil
	}

	// Check the prolog variable is compatible with the function
	if prologVar == nil {
		return nil, errors.New("unexpected prolog variable argument value `nil`")
	}
	prologVarValue := reflect.ValueOf(prologVar)
	prologFuncType := prologVarValue.Type()

	if err := validatePrologVar(fnType, prologFuncType); err != nil {
		return nil, errors.Wrap(err, "prolog variable validation")
	}

	prologFuncType = prologFuncType.Elem().Elem()
	prologVarAddr := (*unsafe.Pointer)(unsafe.Pointer(prologVarValue.Pointer()))

	// Create the hook, store it in the map and return it.
	hook := &Hook{
		symbol:         symbol,
		prologFuncType: prologFuncType,
		prologVarAddr:  prologVarAddr,
	}
	t[symbol] = hook
	return hook, nil
}

// Attach atomically attaches a prolog function to the hook. The hook can be
// disabled with a `nil` prolog value.
func (h *Hook) Attach(prolog PrologCallback) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.New(fmt.Sprintf("set failed: %v", r))
		}
	}()
	addr := h.prologVarAddr
	if prolog == nil {
		// Disable
		atomic.StorePointer(addr, nil)
		return nil
	}
	// 类型检查
	prologType := reflect.TypeOf(prolog)
	if h.prologFuncType != prologType {
		return fmt.Errorf("unexpected prolog type for hook %s: got %T, wanted %s", h, prolog, h.prologFuncType)
	}
	prologValue := reflect.ValueOf(prolog)
	// Create a value having type "pointer to the prolog function"
	ptr := reflect.New(h.prologFuncType)
	// *ptr = prolog
	ptr.Elem().Set(prologValue)
	// Atomically store it: *addr = ptr
	atomic.StorePointer(addr, unsafe.Pointer(ptr.Pointer()))
	return nil
}

// validatePrologVar validates that the prolog variable has the expected type.
// Given a function:
//
//	func F(A, B, C) (R, S, T)
//
// The expected prolog variable to use is:
//
//	var prologVarForF **prolog
func validatePrologVar(fnType, prologVarType reflect.Type) error {
	// Check the prolog variable type is a `**func`.
	if prologVarType.Kind() != reflect.Ptr ||
		prologVarType.Elem().Kind() != reflect.Ptr ||
		prologVarType.Elem().Elem().Kind() != reflect.Func {
		return errors.Errorf("prolog variable type is not a `**func` but `%s`", prologVarType)
	}
	if err := validateProlog(fnType, prologVarType.Elem().Elem()); err != nil {
		return errors.Wrap(err, "prolog function type validation")
	}
	return nil
}

func validateProlog(fnType reflect.Type, prologType reflect.Type) error {
	// Check the prolog is a function
	if prologType.Kind() != reflect.Func {
		return errors.New("the prolog argument type is not a function")
	}
	// Create the list of expected argument types
	expectedArgs := make([]reflect.Type, fnType.NumIn())
	for i := 0; i < fnType.NumIn(); i++ {
		expectedArgs[i] = fnType.In(i)
	}
	// Check the prolog args are pointers to the function args
	if err := validateCallbackArgs(prologType, expectedArgs); err != nil {
		return errors.Wrap(err, "arguments validation")
	}
	// Check the prolog returns two values
	if numPrologOut, numCallbackOut := prologType.NumOut(), 2; numPrologOut != 2 {
		return errors.Errorf("unexpected number result values: expected `%d` but got `%d`", numCallbackOut, numPrologOut)
	}
	// Check the second returned value is an error
	if retType := prologType.Out(1); retType != reflect.TypeOf((*error)(nil)).Elem() {
		return errors.Errorf("unexpected second result value type `%s` instead of `error`", retType)
	}
	// Check the first returned value is the expected epilog type
	epilogType := prologType.Out(0)
	if err := validateEpilog(epilogType, fnType); err != nil {
		return errors.Wrap(err, "epilog validation")
	}
	return nil
}

// validateEpilog validates that the epilog has the expected signature.
func validateEpilog(epilogType reflect.Type, fnType reflect.Type) error {
	// Check the epilog is a function
	if epilogType.Kind() != reflect.Func {
		return errors.New("the epilog argument is not a function")
	}
	// Create the list of argument types
	callbackRetTypes := make([]reflect.Type, fnType.NumOut())
	for i := 0; i < fnType.NumOut(); i++ {
		callbackRetTypes[i] = fnType.Out(i)
	}
	// Check the epilog args are pointers to the function results
	if err := validateCallbackArgs(epilogType, callbackRetTypes); err != nil {
		return errors.Wrap(err, "arguments validation")
	}
	// Check the prolog doesn't return values
	if numOut, expectedOut := epilogType.NumOut(), 0; numOut != expectedOut {
		return errors.Errorf("unexpected number of return values `%d` instead of `%d`", numOut, expectedOut)
	}
	return nil
}

// validateCallbackArgs validates that the callback arguments are pointer to
// the given argument types.
func validateCallbackArgs(callbackType reflect.Type, expectedArgs []reflect.Type) error {
	// Check the callback has the same number of arguments than the function.
	// Note that the method receiver is also in the argument list.
	callbackArgc := callbackType.NumIn()
	if expectedArgc := len(expectedArgs); callbackArgc != expectedArgc {
		return errors.Errorf("unexpected number of arguments: got `%d` instead of `%d`", callbackArgc, expectedArgc)
	}

	if callbackArgc == 0 {
		return nil
	}

	// Check arguments are pointers to the same types than the function arguments.
	// The first argument is the only exception which may be a method receiver.
	//i := 0
	//if callbackType.In(i) == reflect.TypeOf((*MethodReceiver)(nil)).Elem() {
	//	i++
	//}
	for i := 0; i < callbackArgc; i++ {
		// expectedArgType := reflect.PtrTo(expectedArgs[i])
		// callbackArgType := callbackType.In(i)
		// if expectedArgType != callbackArgType {
		// 	return errors.Errorf("argument `%d` has type `%s` instead of `%s`", i, callbackArgType, expectedArgType)
		// }
		expectedArgType := expectedArgs[i]
		expectedArgPrtType := reflect.PtrTo(expectedArgs[i])
		callbackArgType := callbackType.In(i)
		if expectedArgType != callbackArgType && expectedArgPrtType != callbackArgType {
			return fmt.Errorf("argument `%d` has type `%s` instead of `%s`", i, callbackArgType, expectedArgType)
		}
	}
	return nil
}
