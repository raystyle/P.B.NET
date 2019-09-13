package protocol

import (
	"bytes"
	"errors"

	"project/internal/guid"
)

type Role uint8

func (role Role) String() string {
	switch role {
	case Ctrl:
		return "controller"
	case Node:
		return "node"
	case Beacon:
		return "beacon"
	default:
		return "invalid role"
	}
}

func (role Role) Bytes() []byte {
	return []byte{byte(role)}
}

func (role Role) Byte() byte {
	return byte(role)
}

const (
	Ctrl Role = iota
	Node
	Beacon
)

var (
	ErrInvalidRole = errors.New("invalid role")
	ErrInvalidCert = errors.New("invalid certificate")
	ErrAuthFailed  = errors.New("authenticate failed")
)

var (
	CtrlGUID    = bytes.Repeat([]byte{0}, guid.Size)
	AuthSucceed = []byte("success")
)
