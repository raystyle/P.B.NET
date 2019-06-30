package controller

import (
	"net/http"

	"github.com/json-iterator/go"

	"project/internal/bootstrap"
)

func (this *web) h_get_bootstrapper(w h_rw, r *h_r, p h_p) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("hello"))
}

func (this *web) h_trust_node(w h_rw, r *h_r, p h_p) {
	m := &m_trust_node{}
	err := jsoniter.NewDecoder(r.Body).Decode(m)
	if err != nil {
		w.Write([]byte(err.Error()))
	}
	n := &bootstrap.Node{
		Mode:    m.Mode,
		Network: m.Network,
		Address: m.Address,
	}
	err = this.ctx.Trust_Node(n)
	if err != nil {
		w.Write([]byte(err.Error()))
	}
}
