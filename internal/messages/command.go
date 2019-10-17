package messages

import (
	"project/internal/bootstrap"
)

const (
	Test uint32 = 0xFEFFFFFF
)

var (
	TestBytes = []byte{20, 18, 11, 27}
)

const (
	MsgNodeRegisterRequest uint32 = 0x01000000 + iota
	MsgNodeRegisterResponse
	MsgBeaconRegisterRequest
	MsgBeaconRegisterResponse
)

type Bootstrap struct {
	Tag    string
	Mode   bootstrap.Mode
	Config []byte
}
