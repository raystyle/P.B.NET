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

	switch method {
	case "tran":
		tranner, err := lcx.NewTranner("s", dstNetwork, dstAddress, logger.Common, &opts)
		if err != nil {
			log.Fatalln(err)
		}
		start(tranner)
	case "listen":
		listener, err := lcx.NewListener("s", iNetwork, iAddress, logger.Common, &opts)
		if err != nil {
			log.Fatalln(err)
		}
		start(listener)
	case "slave":
		slaver, err := lcx.NewSlaver("s", iNetwork, iAddress, dstNetwork, dstAddress,
			logger.Common, &opts)
		if err != nil {
			log.Fatalln(err)
		}
		start(slaver)
	case "":
		printHelp()
	default:
		fmt.Println("unknown method:", method)
		printHelp()
	}
}

func printHelp() {
	const help = `
 tran:   lcx -m tran -d-addr "192.168.1.2:3389" -l-addr ":8990"
 listen: lcx -m listen -i-addr "1.1.1.1:81" -l-addr "127.0.0.1:8989"
 slave:  lcx -m slave -i-addr "1.1.1.1:81" -d-addr "192.168.1.2:3389"

`
	fmt.Print(help)
	flag.PrintDefaults()
}

func start(module module.Module) {
	err := module.Start()
	if err != nil {
		log.Fatalln(err)
	}
	// stop signal
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	<-signalChan
	module.Stop()
}
