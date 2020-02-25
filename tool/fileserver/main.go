package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"

	"project/internal/logger"
)

func main() {
	var (
		address string
		handler string
		path    string
		cert    string
		key     string
	)
	flag.StringVar(&address, "address", ":8989", "http server port")
	flag.StringVar(&handler, "handler", "/", "web handler")
	flag.StringVar(&path, "path", "web", "file path")
	flag.StringVar(&cert, "cert", "", "tls certificate (pem)")
	flag.StringVar(&key, "key", "", "private key (pem)")
	flag.Parse()

	server := http.Server{
		Addr: address,
	}
	fileServer := http.FileServer(http.Dir(path))
	serveMux := http.NewServeMux()

	// "a" -> "/a/"
	switch handler {
	case "":
		handler = "/"
	case "/":
	default:
		hRune := []rune(handler)
		if len(hRune) == 1 {
			handler = fmt.Sprintf("/%s/", handler)
		} else {
			r := []rune("/")[0]
			if hRune[0] != r {
				hRune = append([]rune("/"), hRune...)
			}
			if hRune[len(hRune)-1] != r {
				hRune = append(hRune, r)
			}
			handler = string(hRune)
		}
	}

	serveMux.HandleFunc(handler, func(w http.ResponseWriter, r *http.Request) {
		log.Print(logger.HTTPRequest(r), "\n\n")
		r.URL.Path = strings.ReplaceAll(r.URL.Path, handler, "/")
		fileServer.ServeHTTP(w, r)
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
