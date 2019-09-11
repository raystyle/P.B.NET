package protocol

import (
	"bytes"
	"errors"

	"project/internal/guid"
)

type Role = uint8

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
	CtrlGUID    = bytes.Repeat([]byte{0}, guid.SIZE)
	AuthSucceed = []byte("success")
)
