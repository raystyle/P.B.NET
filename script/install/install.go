package main

import (
	"os"

	"project/internal/logger"

	"project/script/internal/exec"
	"project/script/internal/log"
)

func init() {
	log.SetSource("build")
}

func main() {
	for _, step := range []func() bool{
		printCurrentDirectory,
		installModule,
		addPatchToGoRoot,
	} {
		if !step() {
			return
		}
	}
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

func installModule() bool {
	log.Println(logger.Info, "start install go module")

	output, code, err := exec.Run("go", "mod", "verify")
	if err != nil {
		log.Println(logger.Error, err)
		return false
	}
	log.Println(logger.Info, output, code)

	log.Println(logger.Info, "add patch to GOROOT/src")
	return true
}

func addPatchToGoRoot() bool {
	log.Println(logger.Info, "start add patch to go root")

	return true
}
