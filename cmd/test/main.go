package main

import (
	"fmt"
	"reflect"
)

func main() {
	type test1 map[string]string
	type test2 map[string]string
	t := test1{}
	t["a"] = "a"
	fmt.Println(reflect.TypeOf(test2(t)))
	fmt.Println(test2(t))
}