package controller

import (
	"net/http"
)

func (this *http_server) h_login(w h_rw, r *h_r, p hr_p) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("hello"))
}

func (this *http_server) h_get_bootstrapper(w h_rw, r *h_r, p hr_p) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("hello"))
}
