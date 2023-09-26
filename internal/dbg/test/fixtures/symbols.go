package main

import (
	"fmt"

	"gni.dev/cmd/internal/dbg/test/fixtures/dup1"
	"gni.dev/cmd/internal/dbg/test/fixtures/dup2"
)

type Struct1 struct {
	a int
	b []int
	c map[string]int
}

func (s *Struct1) Method1() {
	fmt.Println(s.a)
	fmt.Println(s.b)
	fmt.Println(s.c)
}

func func2(b int) {
	fmt.Println(b)
}

func func1(a int) {
	func2(a)
}

func main() {
	func1(1)

	s := &Struct1{a: 1, b: []int{1, 2, 3}, c: map[string]int{"a": 1, "b": 2}}
	s.Method1()

	dup1.Foo()
	dup2.Foo()
}
