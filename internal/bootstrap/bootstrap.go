package bootstrap

import (
	"project/internal/connection"
)

type Node struct {
	Mode    connection.Mode
	Network string
	Address string
}

type Bootstrap interface {
	Generate([]*Node) (string, error)
	Marshal() ([]byte, error)
	Unmarshal([]byte) error
	Resolve() ([]*Node, error)
}
