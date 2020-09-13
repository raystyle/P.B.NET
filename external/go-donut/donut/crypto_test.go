package donut

import (
	"bytes"
	"encoding/binary"
	"log"
	"testing"
)

var (
	// 128-bit master key
	key = []byte{
		0x56, 0x09, 0xE9, 0x68, 0x5F, 0x58, 0xE3, 0x29,
		0x40, 0xEC, 0xEC, 0x98, 0xC5, 0x22, 0x98, 0x2F,
	}
	// 128-bit plain text
	plain = []byte{
		0xB8, 0x23, 0x28, 0x26, 0xFD, 0x5E, 0x40, 0x5E,
		0x69, 0xA3, 0x01, 0xA9, 0x78, 0xEA, 0x7A, 0xD8,
	}
	// 128-bit cipher text
	cipher = []byte{
		0xD5, 0x60, 0x8D, 0x4D, 0xA2, 0xBF, 0x34, 0x7B,
		0xAB, 0xF8, 0x77, 0x2F, 0xDF, 0xED, 0xDE, 0x07,
	}
)

func TestChasLey(t *testing.T) {
	data := plain
	outData := chasKey(key, data)

	if bytes.Equal(outData, cipher) {
		t.Log("chasKey Test Passed")
	} else {
		t.Log("chasKey Test Failed\n", outData, cipher)
		t.Fail()
	}
}

func TestMaru_1(t *testing.T) {
	ivData := []byte{0, 0, 0, 0, 0, 0, 0, 0}
	iv := binary.LittleEndian.Uint64(ivData)
	dllHash := maru([]byte("oleaut32.dll"), iv)
	hash := maru([]byte("SafeArrayCreateVector"), iv) ^ dllHash
	log.Printf("Hash: %x (dllHash was %x)\n", hash, dllHash)

	if 0xbd77af2569689c8a == hash {
		t.Log("Maru Test Passed")
	} else {
		t.Log("Maru Test Failed\n")
		t.Fail()
	}
}

func TestMaru_2(t *testing.T) {
	ivData := []byte{0xEB, 0xA7, 0xF4, 0xDE, 0x07, 0x5B, 0xF8, 0x88}
	iv := binary.LittleEndian.Uint64(ivData)
	dllHash := maru([]byte("kernel32.dll"), iv)
	hash := maru([]byte("Sleep"), iv) ^ dllHash
	log.Printf("Hash: %x (dllHash was %x)\n", hash, dllHash)

	// 0x17, 0xFC, 0xA0, 0x40, 0xD2, 0xBA, 0x66, 0xC7
	if 0xc766bad240a0fc17 == hash {
		t.Log("Maru Test Passed")
	} else {
		t.Log("Maru Test Failed\n")
		t.Fail()
	}
}
