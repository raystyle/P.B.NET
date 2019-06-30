package controller

import (
	"net/http"
)

func (this *http_server) h_get_bootstrapper(w h_rw, r *h_r, p h_p) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("hello"))
}

func (this *http_server) h_trust_node(w h_rw, r *h_r, p h_p) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("hello"))
}
