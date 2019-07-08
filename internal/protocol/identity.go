package protocol

import (
	"bytes"
	"errors"

	"project/internal/guid"
)

type Role = uint8

const (
	CTRL Role = iota
	NODE
	BEACON
	AGENT
)

var (
	ERR_INVALID_ROLE      = errors.New("invalid role")
	ERR_INVALID_CERT_SIZE = errors.New("invalid certificate size")
	ERR_INVALID_CERT      = errors.New("invalid certificate")
	ERR_AUTH_FAILED       = errors.New("authenticate failed")
)

var (
	CTRL_GUID    = bytes.Repeat([]byte{0}, guid.SIZE)
	AUTH_SUCCESS = []byte("success")
)
