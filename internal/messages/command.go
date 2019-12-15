package messages

import (
	"project/internal/convert"
)

// test command
const (
	CMDTest uint32 = 0xFEFFFFFF
)

// commands about register
const (
	CMDRegisterNodeRequest uint32 = 0x01000000 + iota
	CMDRegisterNodeResponse
	CMDRegisterBeaconRequest
	CMDRegisterBeaconResponse
)

// CMDBytes is used to provide parameter to role.Send()
var (
	CMDBytesTest = convert.Uint32ToBytes(CMDTest)
)
