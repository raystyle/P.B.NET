package messages

import (
	"project/internal/convert"
	"project/internal/guid"
)

// RoundTripper is used to set message id.
type RoundTripper interface {
	SetID(id *guid.GUID)
}

// about size
const (
	RandomDataSize  = 4 // make sure the hash of the same message different
	MessageTypeSize = 4 // uint32
	HeaderSize      = RandomDataSize + MessageTypeSize
)

// CMD + Name       means this message without id
// CMD + RT + Name  means this message with id
//
// RT is RoundTripper
// messages with id must send through message manager(Role.messageMgr).

// ---------------------------------------------test-----------------------------------------------
// range 0xF0000000 - 0xFFFFFFFF

// about test role sender.
const (
	CMDTest uint32 = 0xF0000000 + iota
	CMDRTTestRequest
	CMDRTTestResponse
)

// ---------------------------------------------core-----------------------------------------------
// range 0x10000000 - 0x1FFFFFFF

// 0x10 is Controller
// 0x15 is Node
// 0x1A is Beacon

// 0x15001000
// 001 is major
// 000 is minor

// about Controller
const (
	// nop will not do anything, it used for cancel Beacon message that insert to
	// table: beacon_message, controller will replace old message to nop command.
	CMDCtrlNodeNop uint32 = 0x10000000 + iota
	CMDCtrlBeaconNop

	// delete role will delete role key in all Nodes, and all Nodes will disconnect
	// these role if it connect the Node.
	CMDCtrlDeleteNode uint32 = 0x10001000 + iota
	CMDCtrlDeleteBeacon

	// Controller change Beacon communication mode actively.
	CMDCtrlChangeMode uint32 = 0x10002000 + iota
	CMDBeaconChangeModeResult
)

// about Node
const (
	// role's register request and Controller's response.
	CMDNodeRegisterRequestFromNode uint32 = 0x15000000 + iota
	CMDNodeRegisterRequestFromBeacon
	CMDCtrlNodeRegisterResponse
	CMDCtrlBeaconRegisterResponse

	// If current Node doesn't exist role key, it will query Controller.
	// then Controller will answer Node.
	CMDNodeQueryNodeKey uint32 = 0x15001000 + iota
	CMDNodeQueryBeaconKey
	CMDCtrlAnswerNodeKey
	CMDCtrlAnswerBeaconKey

	// Node will send update node message to Controller from Node or Beacon
	CMDNodeUpdateNodeRequestFromNode uint32 = 0x15002000 + iota
	CMDNodeUpdateNodeRequestFromBeacon
	CMDCtrlUpdateNodeResponse
)

// about Beacon
const (
	// ModeChanged is used to notice Controller this Beacon is change to query mode.
	CMDBeaconModeChanged uint32 = 0x1A000000 + iota
)

// -------------------------------------role internal modules--------------------------------------
// range 0x20000000 - 0x2FFFFFFF

// 0x20 is Controller
// 0x25 is Node
// 0x2A is Beacon

// about Controller

// about Node
const (
	CMDNodeLog uint32 = 0x25000000 + iota
)

// about Beacon
const (
	CMDBeaconLog uint32 = 0x2A000000 + iota
)

// --------------------------------------role other modules----------------------------------------
// range 0x30000000 - 0x3FFFFFFF

// simple module
const (
	CMDShellCode uint32 = 0x30000000 + iota
	CMDShellCodeResult
	CMDSingleShell
	CMDSingleShellOutput
)

// ---------------------------------------command to bytes-----------------------------------------
var (
	// -----------------------------------test data----------------------------------
	CMDBTest           = convert.BEUint32ToBytes(CMDTest)
	CMDBRTTestRequest  = convert.BEUint32ToBytes(CMDRTTestRequest)
	CMDBRTTestResponse = convert.BEUint32ToBytes(CMDRTTestResponse)

	// -------------------------------------core-------------------------------------
	// about Controller
	CMDBCtrlNodeNop   = convert.BEUint32ToBytes(CMDCtrlNodeNop)
	CMDBCtrlBeaconNop = convert.BEUint32ToBytes(CMDCtrlBeaconNop)

	CMDBCtrlDeleteNode   = convert.BEUint32ToBytes(CMDCtrlDeleteNode)
	CMDBCtrlDeleteBeacon = convert.BEUint32ToBytes(CMDCtrlDeleteBeacon)

	CMDBCtrlChangeMode         = convert.BEUint32ToBytes(CMDCtrlChangeMode)
	CMDBBeaconChangeModeResult = convert.BEUint32ToBytes(CMDBeaconChangeModeResult)

	// about Node
	CMDBNodeRegisterRequestFromNode   = convert.BEUint32ToBytes(CMDNodeRegisterRequestFromNode)
	CMDBNodeRegisterRequestFromBeacon = convert.BEUint32ToBytes(CMDNodeRegisterRequestFromBeacon)
	CMDBCtrlNodeRegisterResponse      = convert.BEUint32ToBytes(CMDCtrlNodeRegisterResponse)
	CMDBCtrlBeaconRegisterResponse    = convert.BEUint32ToBytes(CMDCtrlBeaconRegisterResponse)

	CMDBNodeQueryNodeKey    = convert.BEUint32ToBytes(CMDNodeQueryNodeKey)
	CMDBNodeQueryBeaconKey  = convert.BEUint32ToBytes(CMDNodeQueryBeaconKey)
	CMDBCtrlAnswerNodeKey   = convert.BEUint32ToBytes(CMDCtrlAnswerNodeKey)
	CMDBCtrlAnswerBeaconKey = convert.BEUint32ToBytes(CMDCtrlAnswerBeaconKey)

	CMDBNodeUpdateNodeRequestFromNode   = convert.BEUint32ToBytes(CMDNodeUpdateNodeRequestFromNode)
	CMDBNodeUpdateNodeRequestFromBeacon = convert.BEUint32ToBytes(CMDNodeUpdateNodeRequestFromBeacon)
	CMDBCtrlUpdateNodeResponse          = convert.BEUint32ToBytes(CMDCtrlUpdateNodeResponse)

	// about Beacon
	CMDBBeaconModeChanged = convert.BEUint32ToBytes(CMDBeaconModeChanged)

	// ----------------------------role internal modules-----------------------------
	// about Node
	CMDBNodeLog = convert.BEUint32ToBytes(CMDNodeLog)

	// about Beacon
	CMDBBeaconLog = convert.BEUint32ToBytes(CMDBeaconLog)

	// ------------------------------role other modules------------------------------
	CMDBShellCode         = convert.BEUint32ToBytes(CMDShellCode)
	CMDBShellCodeResult   = convert.BEUint32ToBytes(CMDShellCodeResult)
	CMDBSingleShell       = convert.BEUint32ToBytes(CMDSingleShell)
	CMDBSingleShellOutput = convert.BEUint32ToBytes(CMDSingleShellOutput)
)
