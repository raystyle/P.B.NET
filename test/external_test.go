package test

import (
	"bytes"
	"fmt"
	"runtime"
	"runtime/debug"
	"sync"
	"testing"
	"time"

	"github.com/pelletier/go-toml"
	"github.com/stretchr/testify/require"

	"project/internal/guid"
)

func TestToml_Negative(t *testing.T) {
	cfg := struct {
		Num uint
	}{}
	err := toml.Unmarshal([]byte(`Num = -1`), &cfg)
	require.Error(t, err)
	require.Equal(t, uint(0), cfg.Num)
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

func TestAll(t *testing.T) {
	TestCtrl_Broadcast_CI(t)
	TestCtrl_Broadcast_IC(t)
	TestCtrl_Broadcast_Mix(t)
	TestCtrl_SendToNode_CI(t)
	TestCtrl_SendToNode_IC(t)
	TestCtrl_SendToNode_Mix(t)
	TestCtrl_SendToBeacon_CI(t)
	TestCtrl_SendToBeacon_IC(t)
	TestCtrl_SendToBeacon_Mix(t)

	TestNode_Send_CI(t)
	TestNode_Send_IC(t)
	TestNode_Send_Mix(t)

	TestBeacon_Send_CI(t)
	TestBeacon_Send_IC(t)
	TestBeacon_Send_Mix(t)
	TestBeacon_Query_CI(t)
	TestBeacon_Query_IC(t)
	TestBeacon_Query_Mix(t)
}

func TestAll_Loop(t *testing.T) {
	loggerLevel = "warning"
	for i := 0; i < 5; i++ {
		fmt.Println("round:", i+1)
		TestAll(t)
		time.Sleep(2 * time.Second)
		runtime.GC()
		debug.FreeOSMemory()
		time.Sleep(10 * time.Second)
		runtime.GC()
		debug.FreeOSMemory()
		time.Sleep(5 * time.Second)
	}
}

func TestAll_Parallel(t *testing.T) {
	loggerLevel = "warning"
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
	for i := 0; i < 10; i++ {
		fmt.Println("round:", i+1)
		TestAll_Parallel(t)
		time.Sleep(2 * time.Second)
		runtime.GC()
		debug.FreeOSMemory()
		time.Sleep(20 * time.Second)
		runtime.GC()
		debug.FreeOSMemory()
		time.Sleep(40 * time.Second)
	}
}
