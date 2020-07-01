package main

import (
	"bytes"
	"compress/flate"
	"crypto/sha256"
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	"project/internal/crypto/aes"
	"project/internal/crypto/lsb"
	"project/internal/system"
)

var (
	encrypt    bool
	decrypt    bool
	textMode   bool
	binaryMode bool
	pngPath    string
	data       string
	password   string
	output     string
)

func main() {
	flag.CommandLine.Usage = printHelp
	flag.BoolVar(&encrypt, "enc", false, "encrypt data to a png file")
	flag.BoolVar(&decrypt, "dec", false, "decrypt data from a png file")
	flag.BoolVar(&textMode, "text", false, "use text mode")
	flag.BoolVar(&binaryMode, "bin", false, "use binary mode")
	flag.StringVar(&pngPath, "png", "", "raw or encrypted png file path")
	flag.StringVar(&data, "data", "", "text message or binary file path")
	flag.StringVar(&password, "pwd", "lsb", "password")
	flag.StringVar(&output, "output", "", "output file path")
	flag.Parse()

	switch {
	case encrypt:
		encryptData()
	case decrypt:
		decryptData()
	default:
		printHelp()
	}
}

func encryptData() {
	// read plain data
	var plainData []byte
	switch {
	case textMode:
		plainData = []byte(data)
	case binaryMode:
		var err error
		plainData, err = ioutil.ReadFile(data) // #nosec
		system.CheckError(err)
	default:
		fmt.Println("select text or binary mode")
		return
	}
	if len(plainData) == 0 {
		fmt.Println("empty data")
		return
	}
	png := readPNG()
	// compress plain data
	buf := bytes.NewBuffer(make([]byte, 0, len(plainData)/2))
	writer, err := flate.NewWriter(buf, flate.BestCompression)
	system.CheckError(err)
	_, err = writer.Write(plainData)
	system.CheckError(err)
	err = writer.Close()
	system.CheckError(err)
	// encrypt
	key, iv := generateAESKeyIV()
	pngEnc, err := lsb.EncryptToPNG(png, buf.Bytes(), key, iv)
	system.CheckError(err)
	// write file
	if output == "" {
		output = "enc.png"
	}
	err = system.WriteFile(output, pngEnc)
	system.CheckError(err)
}

func decryptData() {
	// check mode first
	switch {
	case textMode, binaryMode:
	default:
		fmt.Println("select text or binary mode")
	}
	png := readPNG()
	key, iv := generateAESKeyIV()
	plainData, err := lsb.DecryptFromPNG(png, key, iv)
	system.CheckError(err)
	// decompress plain data
	reader := flate.NewReader(bytes.NewReader(plainData))
	buf := bytes.NewBuffer(make([]byte, 0, len(plainData)*2))
	_, err = buf.ReadFrom(reader)
	system.CheckError(err)
	err = reader.Close()
	system.CheckError(err)
	// handle data
	switch {
	case textMode:
		fmt.Println(buf)
	case binaryMode:
		if output == "" {
			output = "file.txt"
		}
		err = system.WriteFile(output, buf.Bytes())
		system.CheckError(err)
	}
}

func printHelp() {
	exe, err := system.ExecutableName()
	system.CheckError(err)
	const format = `usage:

 [encrypt]
   text mode:   %s -enc -text -png "raw.png" -data "secret" -pwd "pass" -output "enc.png"
   binary mode: %s -enc -bin -png "raw.png" -data "secret.txt" -pwd "pass" -output "enc.png"

 [decrypt]
   text mode:   %s -dec -text -png "enc.png" -pwd "pass"
   binary mode: %s -dec -bin -png "enc.png" -pwd "pass" -output "secret.txt"

`
	fmt.Printf(format, exe, exe, exe, exe)
	flag.CommandLine.SetOutput(os.Stdout)
	flag.PrintDefaults()
}

func readPNG() []byte {
	path := pngPath
	if path == "" {
		switch {
		case encrypt:
			path = "raw.png"
		case decrypt:
			path = "enc.png"
		}
	}
	png, err := ioutil.ReadFile(path) // #nosec
	system.CheckError(err)
	return png
}

func generateAESKeyIV() ([]byte, []byte) {
	hash := sha256.Sum256([]byte(password))
	return hash[:], hash[:aes.IVSize]
}
