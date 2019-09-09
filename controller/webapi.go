package controller

import (
	"encoding/json"

	"github.com/pkg/errors"

	"project/internal/bootstrap"
)

// ------------------------------debug----------------------------------

func (web *web) handleShutdown(w hRW, r *hR, p hP) {
	_ = r.ParseForm()
	errStr := r.FormValue("err")
	_, _ = w.Write([]byte("ok"))
	if errStr != "" {
		web.ctx.Exit(errors.New(errStr))
	} else {
		web.ctx.Exit(nil)
	}
}

func (web *web) handleGetBoot(w hRW, r *hR, p hP) {
	_, _ = w.Write([]byte("hello"))
}

func (web *web) handleTrustNode(w hRW, r *hR, p hP) {
	m := &mTrustNode{}
	err := json.NewDecoder(r.Body).Decode(m)
	if err != nil {
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	n := &bootstrap.Node{
		Mode:    m.Mode,
		Network: m.Network,
		Address: m.Address,
	}
	err = web.ctx.TrustNode(n)
	if err != nil {
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	_, _ = w.Write([]byte("trust ok"))
}
