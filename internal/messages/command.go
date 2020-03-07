package messages

import (
	"project/internal/convert"
)

// about size
const (
	RandomDataSize  = 4 // make sure the hash of the same message different
	MessageTypeSize = 4 // uint32
	HeaderSize      = RandomDataSize + MessageTypeSize
)

// CMDTest is used to test role sender.
const CMDTest uint32 = 0xF0000001

// -------------------------------------------protocol---------------------------------------------
// range 0x10000000 - 0x1FFFFFFF

// role's register request and Controller's response
const (
	CMDNodeRegisterRequest uint32 = 0x10000000 + iota
	CMDNodeRegisterResponse
	CMDBeaconRegisterRequest
	CMDBeaconRegisterResponse
)

// about Node
const (
	// if current Node doesn't exists role key, it will query Controller.
	CMDQueryNodeKey uint32 = 0x10010000 + iota
	CMDQueryBeaconKey
	CMDAnswerNodeKey
	CMDAnswerBeaconKey
)

// about Beacon
const (
	CMDChangeMode uint32 = 0x10020000 + iota
)

// -------------------------------------role internal modules--------------------------------------
// range 0x20000000 - 0x2FFFFFFF

// role's log
const (
	CMDNodeLog uint32 = 0x20000000 + iota
	CMDBeaconLog
)

// -----------------------------------------other modules------------------------------------------
// range 0x30000000 - 0x3FFFFFFF

// simple modules
const (
	CMDExecuteShellCode uint32 = 0x30000000 + iota
	CMDExecuteShellCodeError
	CMDShell
	CMDShellOutput
)

// ---------------------------------------command to bytes-----------------------------------------
var (
	CMDBTest = convert.Uint32ToBytes(CMDTest)

	// about protocol
	CMDBNodeRegisterRequest    = convert.Uint32ToBytes(CMDNodeRegisterRequest)
	CMDBNodeRegisterResponse   = convert.Uint32ToBytes(CMDNodeRegisterResponse)
	CMDBBeaconRegisterRequest  = convert.Uint32ToBytes(CMDBeaconRegisterRequest)
	CMDBBeaconRegisterResponse = convert.Uint32ToBytes(CMDBeaconRegisterResponse)
	CMDBQueryNodeKey           = convert.Uint32ToBytes(CMDQueryNodeKey)
	CMDBQueryBeaconKey         = convert.Uint32ToBytes(CMDQueryBeaconKey)
	CMDBAnswerNodeKey          = convert.Uint32ToBytes(CMDAnswerNodeKey)
	CMDBAnswerBeaconKey        = convert.Uint32ToBytes(CMDAnswerBeaconKey)

	CMDBChangeMode = convert.Uint32ToBytes(CMDChangeMode)

	// role internal modules
	CMDBNodeLog   = convert.Uint32ToBytes(CMDNodeLog)
	CMDBBeaconLog = convert.Uint32ToBytes(CMDBeaconLog)

	// other modules
	CMDBExecuteShellCode      = convert.Uint32ToBytes(CMDExecuteShellCode)
	CMDBExecuteShellCodeError = convert.Uint32ToBytes(CMDExecuteShellCodeError)
	CMDBShell                 = convert.Uint32ToBytes(CMDShell)
	CMDBShellOutput           = convert.Uint32ToBytes(CMDShellOutput)
)
