package controller

import (
	"project/internal/protocol"
)

type syncer struct {
}

func (ctrl *CTRL) syncReceive(token, message []byte) *protocol.SyncResponse {
	return nil
}
