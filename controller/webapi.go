package controller

import (
	"encoding/json"

	"github.com/pkg/errors"

	"project/internal/bootstrap"
)

// ------------------------------debug----------------------------------

func (this *web) h_shutdown(w h_rw, r *h_r, p h_p) {
	_ = r.ParseForm()
	err_str := r.FormValue("err")
	w.Write([]byte("ok"))
	if err_str != "" {
		this.ctx.Exit(errors.New(err_str))
	} else {
		this.ctx.Exit(nil)
	}
}

func (this *web) h_get_boot(w h_rw, r *h_r, p h_p) {
	w.Write([]byte("hello"))
}

func (this *web) h_trust_node(w h_rw, r *h_r, p h_p) {
	m := &m_trust_node{}
	err := json.NewDecoder(r.Body).Decode(m)
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
