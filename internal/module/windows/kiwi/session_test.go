package kiwi

import (
	"fmt"
	"testing"
)

// this test is used to pass golangci-lint structure check
func TestPrintMSV10ListStruct(t *testing.T) {
	fmt.Println(msv10List51Struct)
	fmt.Println(msv10List52Struct)
	fmt.Println(msv10List60Struct)
	fmt.Println(msv10List61Struct)
	fmt.Println(msv10List61AKStruct)
	fmt.Println(msv10List62Struct)
	fmt.Println(msv10List63Struct)
}
