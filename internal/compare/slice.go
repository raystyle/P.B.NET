package compare

// SliceI is an interface for Slice function.
type SliceI interface {
	// Len is the length about this slice.
	Len() int
	// ID is used to identified data.
	ID(i int) string
}

// UniqueSlice is used to compare two slice and return indexes about added and deleted.
// Added are indexes in new slice, deleted are indexes in old slice. Each data must be
// unique in one slice.
func UniqueSlice(new, old SliceI) (added []int, deleted []int) {
	newLen := new.Len()
	oldLen := old.Len()
	// key is data ID, value is data index
	newMap := make(map[string]int, newLen)
	oldMap := make(map[string]int, oldLen)
	for i := 0; i < newLen; i++ {
		newMap[new.ID(i)] = i
	}
	for i := 0; i < oldLen; i++ {
		oldMap[old.ID(i)] = i
	}
	// find added items
	for item, i := range newMap {
		if _, ok := oldMap[item]; !ok {
			added = append(added, i)
		}
	}
	// find deleted items
	for item, i := range oldMap {
		if _, ok := newMap[item]; !ok {
			deleted = append(deleted, i)
		}
	}
	return
}

type stringSlice []string

func (s stringSlice) Len() int {
	return len(s)
}

func (s stringSlice) ID(i int) string {
	return s[i]
}

// UniqueStrings is used to compare two string slice, Added are indexed in new slice,
// deleted are indexes in old slice.each string must be unique in one string slice.
func UniqueStrings(new, old []string) (added []int, deleted []int) {
	return UniqueSlice(stringSlice(new), stringSlice(old))
}
