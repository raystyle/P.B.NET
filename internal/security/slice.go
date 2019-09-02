package security

func FlushBytes(b []byte) {
	for i := 0; i < len(b); i++ {
		b[i] = 0
	}
}
