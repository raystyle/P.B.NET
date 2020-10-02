package vm_test

import (
	"fmt"
	"log"

	"project/external/anko/env"
	"project/external/anko/vm"
)

func Example_vmFunctions() {
	e := env.NewEnv()

	err := e.Define("println", fmt.Println)
	if err != nil {
		log.Fatalf("define error: %v\n", err)
	}

	script := `
func a(b) {
	println(b)
}
a("b")

a = func(b) {
	println(b)
}
a("b")

func(b) {
	println(b)
}("b")

func a() {
	return "a"
}
println(a())

println("")


func fib(n) {
	if (n <= 1) {
		return n
	}
	return fib(n - 1) + fib(n - 2)
}

println(fib(8))
 
 func sum(n...) {
	 t = 0
	 for a in n {
		 t += a
	 }
	 return t
 }
println(sum(1, 2, 3, 4))

func add(a, b) {
	return a + b
}
println(add([1, 2]...))
`

	_, err = vm.Execute(e, nil, script)
	if err != nil {
		log.Fatalf("execute error: %v\n", err)
	}

	// output:
	// b
	// b
	// b
	// a
	//
	// 21
	// 10
	// 3

}

func Example_vmFunctionsScope() {
	e := env.NewEnv()

	err := e.Define("println", fmt.Println)
	if err != nil {
		log.Fatalf("define error: %v\n", err)
	}

	script := `
a = 1
func () {
	a = 2
}()
println(a)

var a = 1
func () {
	a = 2
}()
println(a)

a = 1
func () {
	var a = 2
}()
println(a)

var a = 1
func () {
	var a = 2
}()
println(a)
`

	_, err = vm.Execute(e, nil, script)
	if err != nil {
		log.Fatalf("execute error: %v\n", err)
	}

	// output:
	// 2
	// 2
	// 1
	// 1

}
