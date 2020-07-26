package main

import (
	"bytes"
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
		// set os environment
		err = os.Setenv("HTTP_PROXY", proxyURL)
		if err != nil {
			log.Println(logger.Error, "failed to set os environment:", err)
			return false
		}
	}
	if skipTLSVerify {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // #nosec
	}
	return true

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
			err = filemgr.ZipFileToDir(filename, "temp/dev")
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

func buildSourceCode() bool {
	goRoot, err := config.GoRoot()
	if err != nil {
		log.Println(logger.Error, err)
		return false
	}
	goRoot = filepath.Join(goRoot, "bin")
	// start build
	items := [...]*struct {
		name string
		path string
	}{
		{name: "golint", path: "lint-master/golint"},
		{name: "gocyclo", path: "gocyclo-master/cmd/gocyclo"},
		{name: "gosec", path: "gosec-master/cmd/gosec"},
		{name: "golangci-lint", path: "golangci-lint-master/cmd/golangci-lint"},
	}
	itemsLen := len(items)
	errCh := make(chan error, itemsLen)
	for _, item := range items {
		go func(name, path string) {
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
			// go build -v -i -ldflags "-s -w" -o lint.exe
			args := []string{"build", "-v", "-i", "-ldflags", "-s -w", "-o", binName}
			cmd := exec.Command("go", args...) // #nosec

			cmd.Dir = filepath.Join("F:/dev", path) // TODO replace it
			output, err := cmd.CombinedOutput()
			if err != nil {
				err = fmt.Errorf("%s\n%s", err, output)
				return
			}

			fmt.Println(goRoot)

			// err = os.Rename(filepath.Join(cmd.Dir, ), filepath.Join(goRoot, binName))
			// if err != nil {
			// 	return
			// }

			log.Printf(logger.Info, "build %s successfully", name)
		}(item.name, item.path)
	}
	for i := 0; i < itemsLen; i++ {
		err := <-errCh
		if err != nil {
			log.Println(logger.Error, "failed to build tool:", err)
			return false
		}
	}
	log.Println(logger.Info, "build tools successfully")
	return true
}
