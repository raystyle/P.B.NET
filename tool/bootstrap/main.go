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
		file string
		web  string
		addr string
		cert string
		key  string
	)
	flag.StringVar(&file, "file", "bootstrap.txt", "bootstrap file path")
	flag.StringVar(&web, "web", "/", "serve mux")
	flag.StringVar(&addr, "addr", ":8989", "http server port")
	flag.StringVar(&cert, "cert", "", "tls certificate (pem)")
	flag.StringVar(&key, "key", "", "private key (pem)")
	flag.Parse()

	server := http.Server{Addr: addr}
	mux := http.NewServeMux()
	mux.HandleFunc(web, func(w http.ResponseWriter, r *http.Request) {
		log.Print(logger.HTTPRequest(r), "\n\n")
		w.WriteHeader(http.StatusOK)
		bootstrap, _ := ioutil.ReadFile(file) // #nosec
		_, _ = w.Write(bootstrap)
	})
	server.Handler = mux

	if cert != "" && key != "" {
		err := server.ListenAndServeTLS(cert, key)
		if err != nil {
			log.Fatalln(err)
		}
	} else {
		err := server.ListenAndServe()
		if err != nil {
			log.Fatalln(err)
		}
	}
}
