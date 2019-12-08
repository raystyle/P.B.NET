package code

import (
	"fmt"
	"testing"
)

var ctrlSyncerNeed = []string{
	"nodeSend",
	"nodeAck",
	"beaconSend",
	"beaconAck",
	"query",
}

var nodeSyncerNeed = []string{
	"ctrlSend",
	"ctrlAckToNode",
	"ctrlAckToBeacon",
	"broadcast",
	"answer",

	"nodeSend",
	"nodeAckToCtrl",

	"beaconSend",
	"beaconAckToCtrl",
	"query",
}

func TestGenerateCTRLSyncerCheckGUID(_ *testing.T) {
	fmt.Println("---------------------generate CTRL CheckGUID-------------------------")
	generateCheckGUID(ctrlSyncerNeed)
}

func TestGenerateCTRLSyncerCleanGUID(_ *testing.T) {
	fmt.Println("---------------------generate CTRL CleanGUID-------------------------")
	generateCleanGUID(ctrlSyncerNeed)
}

func TestGenerateCTRLSyncerCleanGUIDMap(_ *testing.T) {
	fmt.Println("--------------------generate CTRL CleanGUIDMap-----------------------")
	generateCleanGUIDMap(ctrlSyncerNeed)
}

func TestGenerateNodeSyncerCheckGUID(_ *testing.T) {
	fmt.Println("---------------------generate node CheckGUID-------------------------")
	generateCheckGUID(nodeSyncerNeed)
}

func TestGenerateNodeSyncerCleanGUID(_ *testing.T) {
	fmt.Println("---------------------generate node CleanGUID-------------------------")
	generateCleanGUID(nodeSyncerNeed)
}

func TestGenerateNodeSyncerCleanGUIDMap(_ *testing.T) {
	fmt.Println("--------------------generate node CleanGUIDMap-----------------------")
	generateCleanGUIDMap(nodeSyncerNeed)
}
