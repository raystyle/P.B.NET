package vm_test

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"project/external/anko/env"
	"project/external/anko/vm"
)

func Example_vmExecuteContext() {
	var waitGroup sync.WaitGroup
	waitGroup.Add(1)
	waitChan := make(chan struct{}, 1)

	e := env.NewEnv()
	sleepMillisecond := func() { time.Sleep(time.Millisecond) }

	err := e.Define("println", fmt.Println)
	if err != nil {
		log.Fatalf("define error: %v\n", err)
	}
	err = e.Define("sleep", sleepMillisecond)
	if err != nil {
		log.Fatalf("define error: %v\n", err)
	}

	script := `
# sleep for 10 seconds
for i = 0; i < 10000; i++ {
	sleep()
}
# the context should cancel before printing the next line
println("this line should not be printed")
`

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		close(waitChan)
		v, err := vm.ExecuteContext(ctx, e, nil, script)
		fmt.Println(v, err)
		waitGroup.Done()
	}()

	<-waitChan
	cancel()

	waitGroup.Wait()

	// output: <nil> execution interrupted
}

func Example_vmEnvDefine() {
	e := env.NewEnv()

	err := e.Define("println", fmt.Println)
	if err != nil {
		log.Fatalf("define error: %v\n", err)
	}

	err = e.Define("a", true)
	if err != nil {
		log.Fatalf("define error: %v\n", err)
	}
	err = e.Define("b", int64(1))
	if err != nil {
		log.Fatalf("define error: %v\n", err)
	}
	err = e.Define("c", 1.1)
	if err != nil {
		log.Fatalf("define error: %v\n", err)
	}
	err = e.Define("d", "d")
	if err != nil {
		log.Fatalf("define error: %v\n", err)
	}
	err = e.Define("e", []interface{}{true, int64(1), 1.1, "d"})
	if err != nil {
		log.Fatalf("define error: %v\n", err)
	}
	err = e.Define("f", map[string]interface{}{"a": true})
	if err != nil {
		log.Fatalf("define error: %v\n", err)
	}

	script := `
println(a)
println(b)
println(c)
println(d)
println(e)
println(f)
`

	_, err = vm.Execute(e, nil, script)
	if err != nil {
		log.Fatalf("execute error: %v\n", err)
	}

	// output:
	// true
	// 1
	// 1.1
	// d
	// [true 1 1.1 d]
	// map[a:true]
}
