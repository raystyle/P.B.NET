package controller

import (
	"project/internal/protocol"
)

// messages from syncer
// <warning> if async must copy slice

func (ctrl *CTRL) handleBroadcast(msg []byte, role protocol.Role, guid []byte) {
	switch role {
	case protocol.Beacon:
		ctrl.handleBeaconBroadcast(msg, guid)
	case protocol.Node:
		ctrl.handleNodeBroadcast(msg, guid)
	default:
		panic("invalid role")
	}
}

func (ctrl *CTRL) handleMessage(msg []byte, role protocol.Role, guid []byte, height uint64) {
	switch role {
	case protocol.Beacon:
		ctrl.handleBeaconMessage(msg, guid, height)
	case protocol.Node:
		ctrl.handleNodeMessage(msg, guid, height)
	default:
		panic("invalid role")
	}
}

func (ctrl *CTRL) handleNodeBroadcast(msg []byte, guid []byte) {

}

func (ctrl *CTRL) handleBeaconBroadcast(msg []byte, guid []byte) {

}

func (ctrl *CTRL) handleNodeMessage(msg []byte, guid []byte, height uint64) {

}

func (ctrl *CTRL) handleBeaconMessage(msg []byte, guid []byte, height uint64) {

}
