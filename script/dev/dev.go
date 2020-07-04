package main

import (
	"bytes"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"project/internal/logger"
	"project/internal/module/compress"
	"project/internal/system"

	"project/script/internal/log"
)

var (
	proxyURL      string
	skipTLSVerify bool
)

func main() {
	usage := "proxy url e.g. \"http://127.0.0.1:8080/\" \"socks5://127.0.0.1:1080/\""
	flag.StringVar(&proxyURL, "proxy-url", "", usage)
	usage = "skip TLS verify"
	flag.BoolVar(&skipTLSVerify, "skip-tls-verify", false, usage)
	flag.Parse()

	log.SetSource("dev")
	for _, step := range []func() bool{
		downloadSourceCode,
		buildTools,
	} {
		if !step() {
			return
		}
	}
	log.Println(logger.Info, "install development tools successfully")
}

func downloadSourceCode() bool {
	// set proxy and TLS config
	tr := http.DefaultTransport.(*http.Transport)
	if proxyURL != "" {
		URL, err := url.Parse(proxyURL)
		if err != nil {
			log.Println(logger.Error, "invalid proxy url:", err)
			return false
		}
		tr.Proxy = http.ProxyURL(URL)
	}
	if skipTLSVerify {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	// download source
	items := [...]*struct {
		name string
		url  string
	}{
		{
			name: "golint",
			url:  "https://github.com/golang/lint/archive/master.zip",
		},
		{
			name: "gocyclo",
			url:  "https://github.com/fzipp/gocyclo/archive/master.zip",
		},
		{
			name: "gosec",
			url:  "https://github.com/securego/gosec/archive/master.zip",
		},
		{
			name: "golangci-lint",
			url:  "https://github.com/golangci/golangci-lint/archive/master.zip",
		},
	}
	itemsLen := len(items)
	errCh := make(chan error, itemsLen)
	for _, item := range items {
		go func(name, url string) {
			var err error
			defer func() { errCh <- err }()
			resp, err := http.Get(url)
			if err != nil {
				return
			}
			defer func() { _ = resp.Body.Close() }()
			// get file size
			size, _ := strconv.Atoi(resp.Header.Get("Content-Length"))
			if size == 0 {
				size = 1 << 20
			}
			buf := bytes.NewBuffer(make([]byte, 0, size))
			// download file
			log.Printf(logger.Info, "downloading %s, url: %s", name, url)
			_, err = io.Copy(buf, resp.Body)
			if err != nil {
				return
			}
			// write file
			filename := fmt.Sprintf("temp/dev/%s.zip", name)
			err = system.WriteFile(filename, buf.Bytes())
			if err != nil {
				return
			}
			// decompress zip file
			err = compress.ZipFileToDir(filename, "temp/dev")
			if err != nil {
				return
			}
			log.Printf(logger.Info, "download %s successfully", name)
		}(item.name, item.url)
	}
	for i := 0; i < itemsLen; i++ {
		err := <-errCh
		if err != nil {
			log.Println(logger.Error, "failed to download source code:", err)
			return false
		}
	}
	log.Println(logger.Info, "download source code successfully")
	return true
}

func buildTools() bool {
	return true
}
