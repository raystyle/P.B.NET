package netx

import (
	"errors"
	"strconv"
)

var (
	ERR_EMPTY_PORT   = errors.New("empty port")
	ERR_INVALID_PORT = errors.New("invalid port")
)

func Check_Port_string(port string) error {
	if port == "" {
		return ERR_EMPTY_PORT
	}
	n, err := strconv.Atoi(port)
	if err != nil {
		return err
	}
	if n < 1 || n > 65535 {
		return ERR_INVALID_PORT
	}
	return nil
}

func Check_Port_int(port int) error {
	if port < 1 || port > 65535 {
		return ERR_INVALID_PORT
	}
	return nil
}
