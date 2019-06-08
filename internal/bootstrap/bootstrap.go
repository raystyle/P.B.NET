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
	// for Controller
	Generate([]*Node) (string, error)
	Marshal() ([]byte, error)
	// for Node Beacon
	Unmarshal([]byte) error
	Resolve() ([]*Node, error)
}
