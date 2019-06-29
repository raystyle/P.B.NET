package main

import (
	"flag"
	"io/ioutil"
	"log"
	"net/http"
)

var data []byte

func main() {
	var err error
	data, err = ioutil.ReadFile("bootstrap.txt")
	if err != nil {
		log.Fatalln(err)
	}
	var port string
	flag.StringVar(&port, "port", "8989", "http server port")
	flag.Parse()
	server := &http.Server{
		Addr: ":" + port,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", info)
	server.Handler = mux
	err = server.ListenAndServe()
	if err != nil {
		log.Fatalln(err)
	}
}

func info(w http.ResponseWriter, r *http.Request) {
	log.Println(r.RemoteAddr)
	log.Println(r.Proto, r.Method, r.URL.String(), "\n", r.Header)
	w.WriteHeader(200)
	_, _ = w.Write(data)
}
