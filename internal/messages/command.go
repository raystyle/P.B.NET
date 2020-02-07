package messages

import (
	"project/internal/convert"
)

// CMDTest is used to test
const CMDTest uint32 = 0xF0000001

// role's register request and Controller's response
const (
	CMDNodeRegisterRequest uint32 = 0x10000000 + iota
	CMDNodeRegisterResponse
	CMDBeaconRegisterRequest
	CMDBeaconRegisterResponse
)

// if current Node doesn't exists role key, it will query Controller.
const (
	CMDNodeQueryNodeKey uint32 = 0x10010000 + iota
	CMDCtrlAnswerNodeKey
	CMDNodeQueryBeaconKey
	CMDCtrlAnswerBeaconKey
)

// about modules
const (
	CMDExecuteShellCode uint32 = 0x20000000 + iota
	CMDExecuteShellCodeError
	CMDShell
	CMDShellOutput
)

// CMD Bytes, role Send need it.
var (
	CMDBTest = convert.Uint32ToBytes(CMDTest)

	CMDBNodeRegisterRequest    = convert.Uint32ToBytes(CMDNodeRegisterRequest)
	CMDBNodeRegisterResponse   = convert.Uint32ToBytes(CMDNodeRegisterResponse)
	CMDBBeaconRegisterRequest  = convert.Uint32ToBytes(CMDBeaconRegisterRequest)
	CMDBBeaconRegisterResponse = convert.Uint32ToBytes(CMDBeaconRegisterResponse)

	CMDBNodeQueryNodeKey    = convert.Uint32ToBytes(CMDNodeQueryNodeKey)
	CMDBCtrlAnswerNodeKey   = convert.Uint32ToBytes(CMDCtrlAnswerNodeKey)
	CMDBNodeQueryBeaconKey  = convert.Uint32ToBytes(CMDNodeQueryBeaconKey)
	CMDBCtrlAnswerBeaconKey = convert.Uint32ToBytes(CMDCtrlAnswerBeaconKey)

	CMDBExecuteShellCode      = convert.Uint32ToBytes(CMDExecuteShellCode)
	CMDBExecuteShellCodeError = convert.Uint32ToBytes(CMDExecuteShellCodeError)
	CMDBShell                 = convert.Uint32ToBytes(CMDShell)
	CMDBShellOutput           = convert.Uint32ToBytes(CMDShellOutput)
)
