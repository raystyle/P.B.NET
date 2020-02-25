package main

import (
	"flag"
	"io/ioutil"
	"log"
	"net/http"

	"project/internal/logger"
)

func main() {
	var (
		address string
		handler string
		file    string
		cert    string
		key     string
	)
	flag.StringVar(&address, "address", ":8989", "http server port")
	flag.StringVar(&handler, "handler", "/", "web handler")
	flag.StringVar(&file, "file", "bootstrap.txt", "bootstrap file path")
	flag.StringVar(&cert, "cert", "", "tls certificate (pem)")
	flag.StringVar(&key, "key", "", "private key (pem)")
	flag.Parse()

	server := http.Server{
		Addr: address,
	}
	serveMux := http.NewServeMux()
	serveMux.HandleFunc(handler, func(w http.ResponseWriter, r *http.Request) {
		log.Print(logger.HTTPRequest(r), "\n\n")
		w.WriteHeader(http.StatusOK)
		bootstrap, _ := ioutil.ReadFile(file) // #nosec
		_, _ = w.Write(bootstrap)
	})
	server.Handler = serveMux

	var err error
	if cert != "" && key != "" {
		err = server.ListenAndServeTLS(cert, key)
	} else {
		err = server.ListenAndServe()
	}
	if err != nil {
		log.Fatalln(err)
	}
}
