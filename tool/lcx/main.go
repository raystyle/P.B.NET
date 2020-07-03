package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"

	"project/internal/logger"
	"project/internal/module"
	"project/internal/module/lcx"
)

var (
	method     string
	dstNetwork string
	dstAddress string
	iNetwork   string
	iAddress   string

	opts lcx.Options
)

func main() {
	usage := "method: tran, listen, slave"
	flag.StringVar(&method, "m", "", usage)
	usage = "tran and slave destination network"
	flag.StringVar(&dstNetwork, "d-net", "tcp", usage)
	usage = "tran and slave destination address"
	flag.StringVar(&dstAddress, "d-addr", "", usage)
	usage = "income listener network and slave"
	flag.StringVar(&iNetwork, "i-net", "tcp", usage)
	usage = "income listener address and slave"
	flag.StringVar(&iAddress, "i-addr", ":0", usage)
	usage = "tran and listener local network"
	flag.StringVar(&opts.LocalNetwork, "l-net", "tcp", usage)
	usage = "tran and listener local address"
	flag.StringVar(&opts.LocalAddress, "l-addr", "127.0.0.1:0", usage)
	usage = "tran and slave, connect target timeout"
	flag.DurationVar(&opts.ConnectTimeout, "connect-timeout", lcx.DefaultConnectTimeout, usage)
	usage = "only slave, connect listener timeout"
	flag.DurationVar(&opts.DialTimeout, "dial-timeout", lcx.DefaultDialTimeout, usage)
	usage = "tran, slave and listener, max connections"
	flag.IntVar(&opts.MaxConns, "max-conn", lcx.DefaultMaxConnections, usage)
	flag.Parse()

	const tag = "s"
	var (
		mod module.Module
		err error
	)
	switch method {
	case "tran":
		mod, err = lcx.NewTranner(tag, dstNetwork, dstAddress, logger.Common, &opts)
	case "listen":
		mod, err = lcx.NewListener(tag, iNetwork, iAddress, logger.Common, &opts)
	case "slave":
		mod, err = lcx.NewSlaver(tag, iNetwork, iAddress, dstNetwork, dstAddress, logger.Common, &opts)
	case "":
		printHelp()
		return
	default:
		fmt.Println("unknown method:", method)
		printHelp()
		return
	}
	checkError(err)
	start(mod)
}

func printHelp() {
	const help = `
 tran:   lcx -m tran -d-addr "192.168.1.2:3389" -l-addr "0.0.0.0:8990"
 listen: lcx -m listen -i-addr "0.0.0.0:81" -l-addr "127.0.0.1:8989"
 slave:  lcx -m slave -i-addr "1.1.1.1:81" -d-addr "192.168.1.2:3389"

`
	fmt.Print(help)
	flag.CommandLine.SetOutput(os.Stdout)
	flag.PrintDefaults()
}

func start(module module.Module) {
	err := module.Start()
	checkError(err)
	// stop signal
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt)
	<-signalCh
	module.Stop()
}

func checkError(err error) {
	if err != nil {
		log.Fatalln(err)
	}
}
