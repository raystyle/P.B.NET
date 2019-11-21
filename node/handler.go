package node

import (
	"project/internal/protocol"
)

type handler struct {
	ctx *Node
}

func (h *handler) HandleSend(s *protocol.Send) {

}

func (h *handler) HandleBroadcast(b *protocol.Broadcast) {

}
