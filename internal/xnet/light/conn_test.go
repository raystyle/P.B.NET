package light

import (
	"context"
	"net"
	"testing"

	"project/internal/testsuite"
)

func TestConn(t *testing.T) {
	server, client := net.Pipe()
	server = Server(context.Background(), server, 0)
	client = Client(context.Background(), client, 0)
	testsuite.ConnSC(t, server, client, true)

	server, client = net.Pipe()
	server = Server(context.Background(), server, 0)
	client = Client(context.Background(), client, 0)
	testsuite.ConnCS(t, client, server, true)
}

func TestConnSCWithContext(t *testing.T) {
	server, client := net.Pipe()
	sCtx, sCancel := context.WithCancel(context.Background())
	defer sCancel()
	server = Server(sCtx, server, 0)
	cCtx, cCancel := context.WithCancel(context.Background())
	defer cCancel()
	client = Client(cCtx, client, 0)
	testsuite.ConnSC(t, server, client, true)
}

func TestConnCSWithContext(t *testing.T) {
	server, client := net.Pipe()
	sCtx, sCancel := context.WithCancel(context.Background())
	defer sCancel()
	server = Server(sCtx, server, 0)
	cCtx, cCancel := context.WithCancel(context.Background())
	defer cCancel()
	client = Client(cCtx, client, 0)
	testsuite.ConnCS(t, client, server, true)
}
