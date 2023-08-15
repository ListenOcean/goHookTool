package hooklib

import (
	"fmt"
	"reflect"
)

func DoHook(funcSym string, replacement interface{}) error {
	hookPoint, err := Find(funcSym)
	if err != nil {
		return err
	}
	if hookPoint == nil {
		return fmt.Errorf("function %s hookpoint not found", funcSym)
	}
	err = hookPoint.Attach(replacement)
	if err != nil {
		return err
	}
	return nil
}

func DoHookWithReflect(funcSym string, replacement interface{}) error {
	hookPoint, err := Find(funcSym)
	if err != nil {
		return err
	}
	if hookPoint == nil {
		return fmt.Errorf("function %s hookpoint not found", funcSym)
	}
	replacementFunc, ok := replacement.(func(args []reflect.Value) (results []reflect.Value))
	if !ok {
		return fmt.Errorf("replacement function's type isn't func(args []reflect.Value) (results []reflect.Value)")
	}
	prolog := reflect.MakeFunc(hookPoint.GetPrologFuncType(), replacementFunc)
	err = hookPoint.Attach(prolog.Interface())
	if err != nil {
		return err
	}
	return nil
}
