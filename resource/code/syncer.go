package code

import (
	"fmt"
	"strings"
)

func generateCheckGUIDSlice(need []string) {
	const template = `
func (syncer *syncer) Check<f>GUIDSlice(slice []byte) bool {
	key := syncer.mapKeyPool.Get().(guid.GUID)
	defer syncer.mapKeyPool.Put(key)
	copy(key[:], slice)
	syncer.<a>GUIDRWM.RLock()
	defer syncer.<a>GUIDRWM.RUnlock()
	_, ok := syncer.<a>GUID[key]
	return !ok
}`
	generateCodeAboutSyncer(template, need)
}

func generateCheckGUID(need []string) {
	const template = `
func (syncer *syncer) Check<f>GUID(guid *guid.GUID, timestamp int64) bool {
	syncer.<a>GUIDRWM.Lock()
	defer syncer.<a>GUIDRWM.Unlock()
	if _, ok := syncer.<a>GUID[*guid]; ok {
		return false
	}
	syncer.<a>GUID[*guid] = timestamp
	return true
}`
	generateCodeAboutSyncer(template, need)
}

func generateCleanGUID(need []string) {
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
	generateCodeAboutSyncer(template, need)
}

func generateCleanGUIDMap(need []string) {
	const template = `
func (syncer *syncer) clean<f>GUIDMap() {
	newMap := make(map[guid.GUID]int64)
	syncer.<a>GUIDRWM.Lock()
	defer syncer.<a>GUIDRWM.Unlock()
	for key, timestamp := range syncer.<a>GUID {
		newMap[key] = timestamp
	}
	syncer.<a>GUID = newMap
}`
	generateCodeAboutSyncer(template, need)
}

func generateCodeAboutSyncer(template string, need []string) {
	for i := 0; i < len(need); i++ {
		a := strings.ReplaceAll(template, "<a>", need[i])
		f := strings.ToUpper(need[i][:1]) + need[i][1:]
		fmt.Println(strings.ReplaceAll(a, "<f>", f))
	}
	fmt.Println()
}
