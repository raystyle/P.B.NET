package node

import (
	"project/internal/protocol"
)

type handler struct {
	ctx *Node
}

func (h *handler) OnSend(s *protocol.Send) {

}

func (h *handler) OnBroadcast(b *protocol.Broadcast) {

}
