package main

import (
	"fmt"
	"net/http"
	"reflect"

	"github.com/ListenOcean/goHookTool/hooklib"

	"github.com/panjf2000/ants"
)

func init() {
	err := hooklib.DoHookWithReflect("runtime.concatstrings", runtimeConcatstringsWithReflect)
	if err != nil {
		fmt.Println(err)
	}
}

func runtimeConcatstringsWithReflect(args []reflect.Value) (results []reflect.Value) {
	fmt.Println("in hook 2")
	epilogFunc := func(s string) {
		fmt.Println(args[1].Interface())
		fmt.Println(s)
	}

	return []reflect.Value{reflect.ValueOf(epilogFunc), reflect.Zero(reflect.TypeOf((*error)(nil)).Elem())}
}

func main() {
	loop()
	local := "asd"
	a := "qwe" + local
	fmt.Println(a)

}

func loop() {
	pool, _ := ants.NewPool(100)
	defer pool.Release()
	for {
		_ = pool.Submit(func() {
			a, b := "a", "b"
			_ = a + b
		})
	}

}

func handler(w http.ResponseWriter, r *http.Request) {
	param1 := r.URL.Query().Get("param1")
	param2 := "p2"
	result := param1 + param2
	fmt.Fprint(w, result)
}
