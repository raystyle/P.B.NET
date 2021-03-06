package config

import (
	"crypto/tls"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"project/internal/logger"
	"project/internal/patch/json"
	"project/internal/system"

	"project/script/internal/log"
)

// Config contains configuration about install, build, develop, test and race.
type Config struct {
	Common struct {
		GoRootLatest  string `json:"go_root_latest"`
		GoRoot1108    string `json:"go_root_1_10_8"`
		ProxyURL      string `json:"proxy_url"`
		SkipTLSVerify bool   `json:"skip_tls_verify"`
	} `json:"common"`

	Install struct {
		DownloadAll bool `json:"download_all"`
	} `json:"install"`

	Build struct {
	} `json:"build"`

	Develop struct {
	} `json:"develop"`

	Test struct {
	} `json:"test"`

	Race struct {
	} `json:"race"`
}

// Load is used to load configuration file.
func Load(path string, config *Config) bool {
	// print current directory
	dir, err := os.Getwd()
	if err != nil {
		log.Println(logger.Error, err)
		return false
	}
	log.Println(logger.Info, "current directory:", dir)
	// load config file
	data, err := ioutil.ReadFile(path) // #nosec
	if err != nil {
		log.Println(logger.Error, "failed to load config file:", err)
		return false
	}
	err = json.Unmarshal(data, config)
	if err != nil {
		log.Println(logger.Error, "failed to load config:", err)
		return false
	}
	log.Println(logger.Info, "load configuration file successfully")
	// check go root path
	goRootLatest := config.Common.GoRootLatest
	if !checkGoRoot(goRootLatest) {
		log.Println(logger.Error, "invalid Go latest root path:", goRootLatest)
		return false
	}
	goRoot1108 := config.Common.GoRoot1108
	if !checkGoRoot(goRoot1108) {
		log.Println(logger.Error, "invalid Go 1.10.8 root path:", goRoot1108)
		return false
	}
	log.Println(logger.Info, "Go latest root path:", goRootLatest)
	log.Println(logger.Info, "Go 1.10.8 root path:", goRoot1108)
	// set proxy and TLS configuration
	tr := http.DefaultTransport.(*http.Transport)
	proxyURL := config.Common.ProxyURL
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
			log.Println(logger.Error, "failed to set os env:", err)
			return false
		}
		log.Println(logger.Info, "set proxy url:", proxyURL)
	}
	if config.Common.SkipTLSVerify {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // #nosec
		log.Println(logger.Warning, "skip tls verify")
	}
	return true
}

// checkGoRoot is used to check go root path is valid.
// it will check go.exe, gofmt.exe and src directory.
func checkGoRoot(path string) bool {
	var (
		goFile    string
		goFmtFile string
	)
	switch runtime.GOOS {
	case "windows":
		goFile = "go.exe"
		goFmtFile = "gofmt.exe"
	default:
		goFile = "go"
		goFmtFile = "gofmt"
	}
	goExist, _ := system.IsExist(filepath.Join(path, "bin/"+goFile))
	goFmtExist, _ := system.IsExist(filepath.Join(path, "bin/"+goFmtFile))
	srcExist, _ := system.IsExist(filepath.Join(path, "src"))
	return goExist && goFmtExist && srcExist
}

// GoRoot is used to get the go root path.
func GoRoot() (string, error) {
	return goEnv("GOROOT")
}

// GoModCache is used to get the go mod cache path.
func GoModCache() (string, error) {
	return goEnv("GOMODCACHE")
}

func goEnv(name string) (string, error) {
	output, err := exec.Command("go", "env", name).CombinedOutput() // #nosec
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}
