package test

import (
	"bytes"
	"crypto/x509"
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"sync"
	"testing"
	"time"

	"github.com/pelletier/go-toml"
	"github.com/stretchr/testify/require"

	"project/internal/crypto/cert"
	"project/internal/guid"
	"project/internal/security"
)

func TestToml_Negative(t *testing.T) {
	cfg := struct {
		Num uint
	}{}
	err := toml.Unmarshal([]byte(`Num = -1`), &cfg)
	require.Error(t, err)
	require.Equal(t, uint(0), cfg.Num)
}

func TestASN1(t *testing.T) {
	pair, err := cert.GenerateCA(nil)
	require.NoError(t, err)

	_, priData := pair.Encode()
	pri, err := x509.ParsePKCS8PrivateKey(priData)
	require.NoError(t, err)

	security.CoverBytes(priData)

	pd, err := x509.MarshalPKCS8PrivateKey(pri)
	require.NoError(t, err)

	_, err = x509.ParsePKCS8PrivateKey(pd)
	require.NoError(t, err)
}

// fast
func BenchmarkBytes_Compare_GUID(b *testing.B) {
	benchmarkBytesCompare(b, guid.Size)
}

// slow
func BenchmarkBytes_Equal_GUID(b *testing.B) {
	benchmarkBytesEqual(b, guid.Size)
}

// slow
func BenchmarkBytes_Compare_1Bytes(b *testing.B) {
	benchmarkBytesCompare(b, 1)
}

// fast
func BenchmarkBytes_Equal_1Bytes(b *testing.B) {
	benchmarkBytesEqual(b, 1)
}

// slow
func BenchmarkBytes_Compare_4Bytes(b *testing.B) {
	benchmarkBytesCompare(b, 4)
}

// fast
func BenchmarkBytes_Equal_4Bytes(b *testing.B) {
	benchmarkBytesEqual(b, 4)
}

// slow
func BenchmarkBytes_Compare_8Bytes(b *testing.B) {
	benchmarkBytesCompare(b, 8)
}

// fast
func BenchmarkBytes_Equal_8Bytes(b *testing.B) {
	benchmarkBytesEqual(b, 8)
}

// slow
func BenchmarkBytes_Compare_10xGUID(b *testing.B) {
	benchmarkBytesCompare(b, 10*guid.Size)
}

// fast
func BenchmarkBytes_Equal_10xGUID(b *testing.B) {
	benchmarkBytesEqual(b, 10*guid.Size)
}

func benchmarkBytesCompare(b *testing.B, size int) {
	aa := bytes.Repeat([]byte{0}, size)
	bb := bytes.Repeat([]byte{0}, size)
	b.ReportAllocs()
	b.ResetTimer()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		bytes.Compare(aa, bb)
	}
	b.StopTimer()
}

func benchmarkBytesEqual(b *testing.B, size int) {
	aa := bytes.Repeat([]byte{0}, size)
	bb := bytes.Repeat([]byte{0}, size)
	b.ReportAllocs()
	b.ResetTimer()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		bytes.Equal(aa, bb)
	}
	b.StopTimer()
}

func testAll() bool {
	// change flag in IDE for test all
	if false {
		return true
	}
	return os.Getenv("test-all") == "true"
}

func TestAll_Ctrl(t *testing.T) {
	if !testAll() {
		return
	}
	TestCtrl_Broadcast_CI(t)
	TestCtrl_Broadcast_IC(t)
	TestCtrl_Broadcast_Mix(t)
	TestCtrl_SendToNode_CI(t)
	TestCtrl_SendToNode_IC(t)
	TestCtrl_SendToNode_Mix(t)
	TestCtrl_SendToBeacon_CI(t)
	TestCtrl_SendToBeacon_IC(t)
	TestCtrl_SendToBeacon_Mix(t)
}

func TestAll_Node(t *testing.T) {
	if !testAll() {
		return
	}
	TestNode_Send_CI(t)
	TestNode_Send_IC(t)
	TestNode_Send_Mix(t)
}

func TestAll_Beacon(t *testing.T) {
	if !testAll() {
		return
	}
	TestBeacon_Send_CI(t)
	TestBeacon_Send_IC(t)
	TestBeacon_Send_Mix(t)
	TestBeacon_Query_CI(t)
	TestBeacon_Query_IC(t)
	TestBeacon_Query_Mix(t)
}

func TestAll(t *testing.T) {
	if !testAll() {
		return
	}
	TestAll_Ctrl(t)
	TestAll_Node(t)
	TestAll_Beacon(t)
}

func TestAll_Loop(t *testing.T) {
	if !testAll() {
		return
	}
	loggerLevel = "warning"

	for i := 0; i < 5; i++ {
		fmt.Println("round:", i+1)
		TestAll(t)
		time.Sleep(3 * time.Second)
		runtime.GC()
		debug.FreeOSMemory()
		time.Sleep(10 * time.Second)
		runtime.GC()
		debug.FreeOSMemory()
		time.Sleep(5 * time.Second)
	}
}

func TestAll_Parallel(t *testing.T) {
	if !testAll() {
		return
	}
	senderTimeout = 15 * time.Second
	syncerExpireTime = 10 * time.Second

	testdataCI := []func(*testing.T){
		TestCtrl_SendToNode_CI,
		TestCtrl_SendToBeacon_CI,
		TestNode_Send_CI,
		TestBeacon_Send_CI,
		TestBeacon_Query_CI,
	}

	testdataIC := []func(*testing.T){
		TestCtrl_SendToNode_IC,
		TestCtrl_SendToBeacon_IC,
		TestNode_Send_IC,
		TestBeacon_Send_IC,
		TestBeacon_Query_IC,
	}

	testdataMix := []func(*testing.T){
		TestCtrl_SendToNode_Mix,
		TestCtrl_SendToBeacon_Mix,
		TestNode_Send_Mix,
		TestBeacon_Send_Mix,
		TestBeacon_Query_Mix,
	}

	wg := sync.WaitGroup{}
	for i := 0; i < len(testdataCI); i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			testdataCI[i](t)
		}(i)
	}
	wg.Wait()
	fmt.Println("Finish CI")

	for i := 0; i < len(testdataIC); i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			testdataIC[i](t)
		}(i)
	}
	wg.Wait()
	fmt.Println("Finish IC")

	for i := 0; i < len(testdataMix); i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			testdataMix[i](t)
		}(i)
	}
	wg.Wait()
	fmt.Println("Finish Mix")
}

func TestAll_Parallel_Loop(t *testing.T) {
	if !testAll() {
		return
	}
	loggerLevel = "warning"

	for i := 0; i < 10; i++ {
		fmt.Println("round:", i+1)
		TestAll_Parallel(t)
		time.Sleep(3 * time.Second)
		runtime.GC()
		debug.FreeOSMemory()
		time.Sleep(20 * time.Second)
		runtime.GC()
		debug.FreeOSMemory()
		time.Sleep(40 * time.Second)
	}
}
