package security

func Flush_Bytes(s []byte) {
	for i := 0; i < len(s); i++ {
		s[i] = 0
	}
}
