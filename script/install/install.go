package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	"project/internal/logger"
	"project/internal/patch/toml"
	"project/internal/system"

	"project/script/internal/config"
	"project/script/internal/exec"
	"project/script/internal/log"
)

var (
	skipDownload bool
	configFile   string

	cfg config.Config
)

func main() {
	log.SetSource("build")

	usage := "skip call go mod download"
	flag.BoolVar(&skipDownload, "skip-download", false, usage)
	usage = "config file path"
	flag.StringVar(&configFile, "config", "config.toml", usage)
	flag.Parse()

	for _, step := range []func() bool{
		printCurrentDirectory,
		loadConfigFile,
		downloadModule,
		verifyModule,
		addPatchToGoRoot,
	} {
		if !step() {
			return
		}
	}

	log.Println(logger.Info, "install successfully")
}

func printCurrentDirectory() bool {
	dir, err := os.Getwd()
	if err != nil {
		log.Println(logger.Error, err)
		return false
	}
	log.Printf(logger.Info, "current directory: \"%s\"", dir)
	return true
}

func loadConfigFile() bool {
	data, err := ioutil.ReadFile("config.toml")
	if err != nil {
		log.Println(logger.Error, "failed to load config file:", err)
		return false
	}
	err = toml.Unmarshal(data, &cfg)
	if err != nil {
		log.Println(logger.Error, "failed to load config:", err)
		return false
	}
	log.Println(logger.Info, "load config file successfully")

	log.Println(logger.Info, "Go latest root path:", cfg.GoRootLatest)
	log.Println(logger.Info, "Go 1.10.8 root path:", cfg.GoRoot1108)
	return true
}

func downloadModule() bool {
	if skipDownload {
		log.Println(logger.Warning, "skip run go mod download")
		return true
	}
	output, code, err := exec.Run("go", "mod", "download")
	if err != nil {
		log.Println(logger.Error, err)
		return false
	}
	log.Println(logger.Info, output)
	if code != 0 {
		return false
	}
	log.Println(logger.Info, "download go module successfully")
	return true
}

func verifyModule() bool {
	output, code, err := exec.Run("go", "mod", "verify")
	if err != nil {
		log.Println(logger.Error, err)
		return false
	}
	output = output[:len(output)-1] // remove the last "\n"
	log.Println(logger.Info, output)
	if code != 0 {
		return false
	}
	log.Println(logger.Info, "verify go module successfully")
	return true
}

func addPatchToGoRoot() bool {
	for _, val := range [...]*struct {
		src  string // patch file in project/patch
		dst  string // relative file path about go root
		note string // error information
	}{
		{
			src:  "patch/crypto/x509/cert_pool_patch.gop",
			dst:  "crypto/x509/cert_pool_patch.go",
			note: "crypto/x509/cert_pool.go",
		},
	} {
		latest := fmt.Sprintf("%s/src/%s", cfg.GoRootLatest, val.dst)
		err := copyFileToGoRoot(val.src, latest)
		if err != nil {
			const format = "failed to add patch file to go latest root path about %s: %s"
			log.Printf(logger.Error, format, val.note, err)
			return false
		}

		go1108 := fmt.Sprintf("%s/src/%s", cfg.GoRoot1108, val.dst)
		err = copyFileToGoRoot(val.src, go1108)
		if err != nil {
			const format = "failed to add patch file to go 1.10.8 root path about %s: %s"
			log.Printf(logger.Error, format, val.note, err)
			return false
		}

		log.Printf(logger.Info, "add patch file %s successfully", val.src)
	}
	log.Println(logger.Info, "add all patch files to go root path successfully")
	return true
}

func copyFileToGoRoot(src, dst string) error {
	data, err := ioutil.ReadFile(src)
	if err != nil {
		return err
	}
	return system.WriteFile(dst, data)
}
