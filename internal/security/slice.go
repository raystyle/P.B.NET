package security

func Flush_Slice(s []byte) {
	for i := 0; i < len(s); i++ {
		s[i] = 0
	}
}
