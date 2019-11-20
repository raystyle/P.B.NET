package main

import (
	"fmt"
	"strings"
)

func main() {
	fmt.Println("---------------------generate CheckGUID-------------------------")
	generateCheckGUID()
	fmt.Println("---------------------generate CleanGUID-------------------------")
	generateCleanGUID()
	fmt.Println("--------------------generate CleanGUIDMap-----------------------")
	generateCleanGUIDMap()
}

func generateCheckGUID() {
	const template = `
func (syncer *syncer) Check<f>GUID(guid []byte, add bool, timestamp int64) bool {
	key := base64.StdEncoding.EncodeToString(guid)
	if add {
		syncer.<a>GUIDRWM.Lock()
		defer syncer.<a>GUIDRWM.Unlock()
		if _, ok := syncer.<a>GUID[key]; ok {
			return false
		} else {
			syncer.<a>GUID[key] = timestamp
			return true
		}
	} else {
		syncer.<a>GUIDRWM.RLock()
		defer syncer.<a>GUIDRWM.RUnlock()
		_, ok := syncer.<a>GUID[key]
		return !ok
	}
}`
	generateCode(template)
}

func generateCleanGUID() {
	const template = `
func (syncer *syncer) clean<f>GUID(now int64) {
	syncer.<a>GUIDRWM.Lock()
	defer syncer.<a>GUIDRWM.Unlock()
    for key, timestamp := range syncer.<a>GUID {
		if math.Abs(float64(now-timestamp)) > syncer.expireTime {
			delete(syncer.<a>GUID, key)
		}
	}
}`
	generateCode(template)
}

func generateCleanGUIDMap() {
	const template = `
func (syncer *syncer) clean<f>GUIDMap() {
	syncer.<a>GUIDRWM.Lock()
	defer syncer.<a>GUIDRWM.Unlock()
	newMap := make(map[string]int64)
	for key, timestamp := range syncer.<a>GUID {
		newMap[key] = timestamp
	}
	syncer.<a>GUID = newMap
}`
	generateCode(template)
}

func generateCode(template string) {
	var need = [...]string{
		"ctrlSend",
		"nodeAckCtrl",
		"beaconAckCtrl",
		"broadcast",

		"nodeSend",
		"ctrlAckNode",

		"beaconSend",
		"ctrlAckBeacon",
		"beaconQuery",
		"ctrlAnswer",
	}
	for i := 0; i < len(need); i++ {
		a := strings.ReplaceAll(template, "<a>", need[i])
		f := strings.ToUpper(need[i][:1]) + need[i][1:]
		fmt.Println(strings.ReplaceAll(a, "<f>", f))
	}
}
