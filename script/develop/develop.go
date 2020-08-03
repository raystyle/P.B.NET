package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"

	"project/internal/logger"
	"project/internal/module/filemgr"
	"project/internal/system"

	"project/script/internal/config"
	"project/script/internal/log"
)

const developDir = "temp/develop"

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

	log.SetSource("develop")
	for _, step := range []func() bool{
		downloadSourceCode,
		extractSourceCode,
		buildSourceCode,
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
		// set os environment for build
		err = os.Setenv("HTTP_PROXY", proxyURL)
		if err != nil {
			log.Println(logger.Error, "failed to set os environment:", err)
			return false
		}
	}
	if skipTLSVerify {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // #nosec
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
			resp, err := http.Get(url) // #nosec
			if err != nil {
				return
			}
			defer func() { _ = resp.Body.Close() }()
			// get file size
			size, _ := strconv.Atoi(resp.Header.Get("Content-Length"))
			if size == 0 {
				size = 1024 * 1024
			}
			buf := bytes.NewBuffer(make([]byte, 0, size))
			// download file
			log.Printf(logger.Info, "downloading %s url: %s", name, url)
			_, err = io.Copy(buf, resp.Body)
			if err != nil {
				return
			}
			// write file
			filename := fmt.Sprintf(developDir+"/%s.zip", name)
			err = system.WriteFile(filename, buf.Bytes())
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
	log.Println(logger.Info, "download all source code successfully")
	return true
}

func extractSourceCode() bool {
	items := [...]*struct {
		name string
		dir  string
	}{
		{name: "golint", dir: "lint-master"},
		{name: "gocyclo", dir: "gocyclo-master"},
		{name: "gosec", dir: "gosec-master"},
		{name: "golangci-lint", dir: "golangci-lint-master"},
	}
	itemsLen := len(items)
	errCh := make(chan error, itemsLen)
	for _, item := range items {
		go func(name, dir string) {
			var err error
			defer func() { errCh <- err }()
			// clean directory
			dir = filepath.Join(developDir, dir)
			exist, err := system.IsExist(dir)
			if err != nil {
				return
			}
			if exist {
				err = os.RemoveAll(dir)
				if err != nil {
					return
				}
			}
			src := developDir + "/" + name + ".zip"
			// extract files
			ec := func(_ context.Context, typ uint8, e error, _ *filemgr.SrcDstStat) uint8 {
				err = e
				return filemgr.ErrCtrlOpCancel
			}
			err = filemgr.UnZip(ec, src, developDir)
			if err != nil {
				return
			}
			// delete zip file
			err = os.Remove(src)
			if err != nil {
				return
			}
			log.Printf(logger.Info, "extract %s.zip successfully", name)
		}(item.name, item.dir)
	}
	for i := 0; i < itemsLen; i++ {
		err := <-errCh
		if err != nil {
			log.Println(logger.Error, "failed to extract source code:", err)
			return false
		}
	}
	log.Println(logger.Info, "extract all source code successfully")
	return true
}

func buildSourceCode() bool {
	goRoot, err := config.GoRoot()
	if err != nil {
		log.Println(logger.Error, err)
		return false
	}
	goRoot = filepath.Join(goRoot, "bin")
	// start build
	items := [...]*struct {
		name  string
		dir   string
		build string
	}{
		{name: "golint", dir: "lint-master", build: "golint"},
		{name: "gocyclo", dir: "gocyclo-master", build: "cmd/gocyclo"},
		{name: "gosec", dir: "gosec-master", build: "cmd/gosec"},
		{name: "golangci-lint", dir: "golangci-lint-master", build: "cmd/golangci-lint"},
	}
	itemsLen := len(items)
	errCh := make(chan error, itemsLen)
	for _, item := range items {
		go func(name, dir, build string) {
			var err error
			defer func() { errCh <- err }()
			var binName string
			switch runtime.GOOS {
			case "windows":
				binName = name + ".exe"
			case "linux":
				binName = name
			default:
				err = errors.New("unsupported platform: " + runtime.GOOS)
				return
			}
			buildPath := filepath.Join(developDir, dir, build)
			// go build -v -i -ldflags "-s -w" -o lint.exe
			args := []string{"build", "-v", "-i", "-ldflags", "-s -w", "-o", binName}
			cmd := exec.Command("go", args...) // #nosec
			cmd.Dir = buildPath
			writer := logger.Wrap(logger.Info, "develop", logger.Common).Writer()
			cmd.Stdout = writer
			cmd.Stderr = writer
			err = cmd.Run()
			if err != nil {
				return
			}
			// move binary file to GOROOT
			binPath := filepath.Join(buildPath, binName)
			// TODO use filemgr.Move()
			err = os.Rename(binPath, filepath.Join(developDir, binName))
			if err != nil {
				return
			}
			// delete source code directory
			err = os.RemoveAll(filepath.Join(developDir, dir))
			if err != nil {
				return
			}
			log.Printf(logger.Info, "build development tool %s successfully", name)
		}(item.name, item.dir, item.build)
	}
	for i := 0; i < itemsLen; i++ {
		err := <-errCh
		if err != nil {
			log.Println(logger.Error, "failed to build development tool:", err)
			return false
		}
	}
	log.Println(logger.Info, "build all development tools successfully")
	return true
}
