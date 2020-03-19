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

// CMDTest is used to test role sender.
const (
	CMDTest uint32 = 0xF0000000 + iota
	CMDRTTestRequest
	CMDRTTestResponse
)

// ---------------------------------------------core-----------------------------------------------
// range 0x10000000 - 0x1FFFFFFF

// nop will not do anything, it used for cancel Beacon message that insert to
// table: beacon_message, controller will replace old message to nop command.
const (
	CMDNodeNop uint32 = 0x10000000 + iota
	CMDBeaconNop
)

// role's register request and Controller's response.
const (
	CMDNodeRegisterRequest uint32 = 0x10010000 + iota
	CMDNodeRegisterResponse
	CMDBeaconRegisterRequest
	CMDBeaconRegisterResponse
)

// about Node

// If current Node doesn't exist role key, it will query Controller.
// then Controller will answer Node.
const (
	CMDNodeQueryNodeKey uint32 = 0x10020000 + iota
	CMDNodeQueryBeaconKey
	CMDNodeAnswerNodeKey
	CMDNodeAnswerBeaconKey
	CMDNodeDeleteNode
	CMDNodeDeleteBeacon
)

// about Beacon

// driver
const (
	CMDBeaconChangeMode uint32 = 0x10030000 + iota
	CMDBeaconChangeModeResult
	CMDBeaconModeChanged
)

// -------------------------------------role internal modules--------------------------------------
// range 0x20000000 - 0x2FFFFFFF

// role's log
const (
	CMDNodeLog uint32 = 0x20000000 + iota
	CMDBeaconLog
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
	// about test data
	CMDBTest           = convert.Uint32ToBytes(CMDTest)
	CMDBRTTestRequest  = convert.Uint32ToBytes(CMDRTTestRequest)
	CMDBRTTestResponse = convert.Uint32ToBytes(CMDRTTestResponse)

	// about core
	CMDBNodeNop   = convert.Uint32ToBytes(CMDNodeNop)
	CMDBBeaconNop = convert.Uint32ToBytes(CMDBeaconNop)

	CMDBNodeRegisterRequest    = convert.Uint32ToBytes(CMDNodeRegisterRequest)
	CMDBNodeRegisterResponse   = convert.Uint32ToBytes(CMDNodeRegisterResponse)
	CMDBBeaconRegisterRequest  = convert.Uint32ToBytes(CMDBeaconRegisterRequest)
	CMDBBeaconRegisterResponse = convert.Uint32ToBytes(CMDBeaconRegisterResponse)

	CMDBNodeQueryNodeKey    = convert.Uint32ToBytes(CMDNodeQueryNodeKey)
	CMDBNodeQueryBeaconKey  = convert.Uint32ToBytes(CMDNodeQueryBeaconKey)
	CMDBNodeAnswerNodeKey   = convert.Uint32ToBytes(CMDNodeAnswerNodeKey)
	CMDBNodeAnswerBeaconKey = convert.Uint32ToBytes(CMDNodeAnswerBeaconKey)
	CMDBNodeDeleteNode      = convert.Uint32ToBytes(CMDNodeDeleteNode)
	CMDBNodeDeleteBeacon    = convert.Uint32ToBytes(CMDNodeDeleteBeacon)

	CMDBBeaconChangeMode       = convert.Uint32ToBytes(CMDBeaconChangeMode)
	CMDBBeaconChangeModeResult = convert.Uint32ToBytes(CMDBeaconChangeModeResult)
	CMDBBeaconModeChanged      = convert.Uint32ToBytes(CMDBeaconModeChanged)

	// role internal modules
	CMDBNodeLog   = convert.Uint32ToBytes(CMDNodeLog)
	CMDBBeaconLog = convert.Uint32ToBytes(CMDBeaconLog)

	// role other modules
	CMDBShellCode         = convert.Uint32ToBytes(CMDShellCode)
	CMDBShellCodeResult   = convert.Uint32ToBytes(CMDShellCodeResult)
	CMDBSingleShell       = convert.Uint32ToBytes(CMDSingleShell)
	CMDBSingleShellOutput = convert.Uint32ToBytes(CMDSingleShellOutput)
)
