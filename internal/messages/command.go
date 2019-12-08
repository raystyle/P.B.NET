package messages

import (
	"project/internal/convert"
)

const (
	CMDTest uint32 = 0xFEFFFFFF
)

const (
	CMDRegisterNodeRequest uint32 = 0x01000000 + iota
	CMDRegisterNodeResponse
	CMDRegisterBeaconRequest
	CMDRegisterBeaconResponse
)

var (
	CMDBytesTest = convert.Uint32ToBytes(CMDTest)
)
