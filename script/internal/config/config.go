package config

import (
	"os/exec"
	"strings"
)

// Config contains configuration about install, build, test and race.
type Config struct {
	Common struct {
		GoRootLatest  string `toml:"go_root_latest"`
		GoRoot1108    string `toml:"go_root_1_10_8"`
		ProxyURL      string `toml:"proxy_url"`
		SkipTLSVerify bool   `toml:"skip_tls_verify"`
	} `toml:"common"`

	Install struct {
		DownloadAll bool `toml:"download_all"`
	} `toml:"install"`

	Build struct {
	} `toml:"build"`

	Develop struct {
	} `toml:"develop"`

	Test struct {
	} `toml:"test"`

	Race struct {
	} `toml:"race"`
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
