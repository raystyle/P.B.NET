package config

import (
	"os/exec"
	"strings"
)

// Config contains configuration about install, build, test and race.
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
