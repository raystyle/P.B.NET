package controller

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

func get_bootstrapper(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("hello"))
}
