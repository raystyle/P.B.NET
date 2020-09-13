package role

import (
	"fmt"
	"testing"
)

var (
	ctrlSyncerNeed = []string{
		"nodeSend", "nodeAck", "beaconSend", "beaconAck", "query",
	}
	nodeSyncerNeed = []string{
		"sendToNode", "sendToBeacon", "ackToNode", "ackToBeacon", "broadcast", "answer",
		"nodeSend", "nodeAck", "beaconSend", "beaconAck", "query",
	}
	beaconSyncerNeed = []string{
		"sendToBeacon", "ackToBeacon", "answer",
	}
)

func TestGenerateControllerSyncer(t *testing.T) {
	t.Run("CheckGUIDSlice", func(t *testing.T) {
		fmt.Println("-------------generate Controller syncer CheckGUIDSlice------------------")
		generateCheckGUIDSlice(ctrlSyncerNeed)
	})

	t.Run("CheckGUID", func(t *testing.T) {
		fmt.Println("----------------generate Controller syncer CheckGUID--------------------")
		generateCheckGUID(ctrlSyncerNeed)
	})

	t.Run("CleanGUID", func(t *testing.T) {
		fmt.Println("----------------generate Controller syncer CleanGUID--------------------")
		generateCleanGUID(ctrlSyncerNeed)
	})

	t.Run("CleanGUIDMap", func(t *testing.T) {
		fmt.Println("---------------generate Controller syncer CleanGUIDMap------------------")
		generateCleanGUIDMap(ctrlSyncerNeed)
	})
}

func TestGenerateNodeSyncer(t *testing.T) {
	t.Run("CheckGUIDSlice", func(t *testing.T) {
		fmt.Println("-----------------generate Node syncer CheckGUIDSlice--------------------")
		generateCheckGUIDSlice(nodeSyncerNeed)
	})

	t.Run("CheckGUID", func(t *testing.T) {
		fmt.Println("-------------------generate Node syncer CheckGUID-----------------------")
		generateCheckGUID(nodeSyncerNeed)
	})

	t.Run("CleanGUID", func(t *testing.T) {
		fmt.Println("-------------------generate Node syncer CleanGUID-----------------------")
		generateCleanGUID(nodeSyncerNeed)
	})

	t.Run("CleanGUIDMap", func(t *testing.T) {
		fmt.Println("------------------generate Node syncer CleanGUIDMap---------------------")
		generateCleanGUIDMap(nodeSyncerNeed)
	})
}

func TestGenerateBeaconSyncer(t *testing.T) {
	t.Run("CheckGUIDSlice", func(t *testing.T) {
		fmt.Println("----------------generate Beacon syncer CheckGUIDSlice-------------------")
		generateCheckGUIDSlice(beaconSyncerNeed)
	})

	t.Run("CheckGUID", func(t *testing.T) {
		fmt.Println("------------------generate Beacon syncer CheckGUID----------------------")
		generateCheckGUID(beaconSyncerNeed)
	})

	t.Run("CleanGUID", func(t *testing.T) {
		fmt.Println("------------------generate Beacon syncer CleanGUID----------------------")
		generateCleanGUID(beaconSyncerNeed)
	})

	t.Run("CleanGUIDMap", func(t *testing.T) {
		fmt.Println("-----------------generate Beacon syncer CleanGUIDMap--------------------")
		generateCleanGUIDMap(beaconSyncerNeed)
	})
}
