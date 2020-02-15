package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"sync"

	"project/internal/module/shell"
)

func main() {
	terminal, err := shell.NewTerminal(false)
	if err != nil {
		log.Fatal(err)
	}
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(os.Stdout, terminal)
	}()
	go func() {
		_, _ = io.Copy(terminal, os.Stdin)
	}()
	go func() {
		signalCh := make(chan os.Signal, 1)
		signal.Notify(signalCh, os.Interrupt)
		for {
			<-signalCh
			err = terminal.Interrupt()
			if err != nil {
				log.Fatal(err)
			}
		}
	}()
	wg.Wait()
	fmt.Println("exit")
}
