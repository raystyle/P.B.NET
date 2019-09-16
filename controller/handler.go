package controller

import (
	"github.com/vmihailenco/msgpack/v4"

	"project/internal/convert"
	"project/internal/logger"
	"project/internal/messages"
	"project/internal/protocol"
	"project/internal/xpanic"
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

// height start at 0
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

func (ctrl *CTRL) handleLogf(l logger.Level, format string, log ...interface{}) {
	ctrl.Printf(l, "handler", format, log...)
}

func (ctrl *CTRL) handleLog(l logger.Level, log ...interface{}) {
	ctrl.Print(l, "handler", log...)
}

func (ctrl *CTRL) handleLogln(l logger.Level, log ...interface{}) {
	ctrl.Println(l, "handler", log...)
}

func (ctrl *CTRL) handleNodeBroadcast(msg []byte, guid []byte) {
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error("handler panic:", r)
			ctrl.handleLog(logger.Fatal, err)
		}
	}()
	if len(msg) < 4 {
		ctrl.handleLogf(logger.Exploit, "node %X broadcast message with invalid size",
			guid)
		return
	}
	switch convert.BytesToUint32(msg[:4]) {

	case messages.Test:
		var testMsg []byte
		err := msgpack.Unmarshal(msg[4:], &testMsg)
		if err != nil {
			ctrl.handleLogf(logger.Exploit, "node %X broadcast invalid test message: %X",
				guid, msg[4:])
			return
		}
		if ctrl.Debug.NodeBroadcastChan != nil {
			ctrl.Debug.NodeBroadcastChan <- testMsg
		}
		ctrl.handleLogf(logger.Debug, "node %X broadcast test message: %s",
			guid, string(testMsg))
	default:
		ctrl.handleLogf(logger.Exploit, "node %X broadcast unknown message: %X",
			guid, msg)
	}
}

func (ctrl *CTRL) handleBeaconBroadcast(msg []byte, guid []byte) {
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error("handler panic:", r)
			ctrl.handleLog(logger.Fatal, err)
		}
	}()
	if len(msg) < 4 {
		ctrl.handleLogf(logger.Exploit, "beacon %X broadcast message with invalid size",
			guid)
		return
	}
	switch convert.BytesToUint32(msg[:4]) {

	case messages.Test:
		ctrl.handleLogf(logger.Debug, "beacon %X broadcast test message: %s",
			guid, string(msg[4:]))
	default:
		ctrl.handleLogf(logger.Exploit, "beacon %X broadcast unknown message: %X",
			guid, msg)
	}
}

func (ctrl *CTRL) handleNodeMessage(msg []byte, guid []byte, height uint64) {
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error("handler panic:", r)
			ctrl.handleLog(logger.Fatal, err)
		}
	}()
	if len(msg) < 4 {
		ctrl.handleLogf(logger.Exploit, "node %X send message with invalid size height: %d",
			guid, height)
		return
	}
	switch convert.BytesToUint32(msg[:4]) {

	case messages.Test:
		var testMsg []byte
		err := msgpack.Unmarshal(msg[4:], &testMsg)
		if err != nil {
			ctrl.handleLogf(logger.Exploit, "node %X send invalid test message: %X",
				guid, msg[4:])
			return
		}
		if ctrl.Debug.NodeSyncSendChan != nil {
			ctrl.Debug.NodeSyncSendChan <- testMsg
		}
		ctrl.handleLogf(logger.Debug, "node %X send test message: %s height: %d",
			guid, string(testMsg), height)
	default:
		ctrl.handleLogf(logger.Exploit, "node %X send unknown message: %X height: %d",
			guid, msg, height)
	}
}

func (ctrl *CTRL) handleBeaconMessage(msg []byte, guid []byte, height uint64) {
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error("handler panic:", r)
			ctrl.handleLog(logger.Fatal, err)
		}
	}()
	if len(msg) < 4 {
		ctrl.handleLogf(logger.Exploit, "beacon %X send message with invalid size height: %d",
			guid, height)
		return
	}
	switch convert.BytesToUint32(msg[:4]) {

	case messages.Test:
		ctrl.handleLogf(logger.Debug, "beacon %X send test message: %s height: %d",
			guid, string(msg[4:]), height)
	default:
		ctrl.handleLogf(logger.Exploit, "beacon %X send unknown message: %X height: %d",
			guid, msg, height)
	}
}
