package messages

import (
	"project/internal/convert"
)

// test command
const (
	CMDTest uint32 = 0xF0000001
)

// commands about role register
const (
	CMDNodeRegisterRequest uint32 = 0x00000000 + iota
	CMDNodeRegisterResponse
	CMDBeaconRegisterRequest
	CMDBeaconRegisterResponse
)

// commands about modules
const (
	CMDExecuteShellCode uint32 = 0x10000000 + iota
)

// CMD Bytes, role Send need it
var (
	CMDBTest                   = convert.Uint32ToBytes(CMDTest)
	CMDBNodeRegisterRequest    = convert.Uint32ToBytes(CMDNodeRegisterRequest)
	CMDBNodeRegisterResponse   = convert.Uint32ToBytes(CMDNodeRegisterResponse)
	CMDBBeaconRegisterRequest  = convert.Uint32ToBytes(CMDBeaconRegisterRequest)
	CMDBBeaconRegisterResponse = convert.Uint32ToBytes(CMDBeaconRegisterResponse)
	CMDBExecuteShellCode       = convert.Uint32ToBytes(CMDExecuteShellCode)
)
