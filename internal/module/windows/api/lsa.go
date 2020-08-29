package api

// LSAUnicodeString is used by various Local Security Authority (LSA) functions
// to specify a Unicode string.
type LSAUnicodeString struct {
	Length        uint16
	MaximumLength uint16
	Buffer        uintptr
}
