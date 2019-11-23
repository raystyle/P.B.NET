package main

import (
	"fmt"
	"strings"
)

func generateCheckGUID() {
	const template = `
func (syncer *syncer) Check<f>GUID(guid []byte, add bool, timestamp int64) bool {
	dst := syncer.hexPool.Get().([]byte)
	hex.Encode(dst, guid)
	key := string(dst)
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
	generateCodeAboutSyncer(template)
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
	generateCodeAboutSyncer(template)
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
	generateCodeAboutSyncer(template)
}

func generateCodeAboutSyncer(template string) {
	var need = [...]string{
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
	for i := 0; i < len(need); i++ {
		a := strings.ReplaceAll(template, "<a>", need[i])
		f := strings.ToUpper(need[i][:1]) + need[i][1:]
		fmt.Println(strings.ReplaceAll(a, "<f>", f))
	}
}
