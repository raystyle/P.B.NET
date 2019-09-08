package controller

import (
	"encoding/json"

	"github.com/pkg/errors"

	"project/internal/bootstrap"
)

// ------------------------------debug----------------------------------

func (web *web) hShutdown(w hRW, r *hR, p hP) {
	_ = r.ParseForm()
	errStr := r.FormValue("err")
	_, _ = w.Write([]byte("ok"))
	if errStr != "" {
		web.ctx.Exit(errors.New(errStr))
	} else {
		web.ctx.Exit(nil)
	}
}

func (web *web) hGetBoot(w hRW, r *hR, p hP) {
	_, _ = w.Write([]byte("hello"))
}

func (web *web) hTrustNode(w hRW, r *hR, p hP) {
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
