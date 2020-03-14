package guid

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/convert"
	"project/internal/patch/monkey"
	"project/internal/random"
	"project/internal/testsuite"
)

func TestGUID(t *testing.T) {
	t.Run("Write", func(t *testing.T) {
		expect := bytes.Repeat([]byte{1}, Size)
		guid := GUID{}
		err := guid.Write(expect)
		require.NoError(t, err)
		require.Equal(t, expect, guid[:])

		// invalid slice size
		err = guid.Write(bytes.Repeat([]byte{1}, Size-1))
		require.Error(t, err)
	})

	t.Run("Print", func(t *testing.T) {
		guid := GUID{}
		copy(guid[Size/2:], bytes.Repeat([]byte{10}, Size/2))
		buf := bytes.Buffer{}
		buf.WriteString("GUID: ")
		buf.WriteString(strings.Repeat("00", Size/2))
		buf.WriteString(strings.Repeat("0A", Size/2))
		require.Equal(t, buf.String(), guid.Print())
	})

	t.Run("Hex", func(t *testing.T) {
		guid := GUID{}
		copy(guid[Size/2:], bytes.Repeat([]byte{10}, Size/2))
		buf := bytes.Buffer{}
		buf.WriteString(strings.Repeat("00", Size/2))
		buf.WriteString(strings.Repeat("0A", Size/2))
		require.Equal(t, buf.String(), guid.Hex())
	})

	t.Run("Timestamp", func(t *testing.T) {
		now := time.Now().Unix()
		guid := GUID{}
		copy(guid[20:28], convert.Int64ToBytes(now))
		require.Equal(t, now, guid.Timestamp())
	})

	t.Run("MarshalJSON", func(t *testing.T) {
		guid := GUID{}
		data := bytes.Repeat([]byte{1}, Size)
		copy(guid[:], data)
		data, err := guid.MarshalJSON()
		require.NoError(t, err)
		// "0101...0101"
		expected := fmt.Sprintf("\"%s\"", strings.Repeat("01", Size))
		require.Equal(t, expected, string(data))
	})

	t.Run("UnmarshalJSON", func(t *testing.T) {
		data := []byte(fmt.Sprintf("\"%s\"", strings.Repeat("01", Size)))
		guid := GUID{}
		err := guid.UnmarshalJSON(data)
		require.NoError(t, err)
		expected := bytes.Repeat([]byte{1}, Size)
		require.Equal(t, expected, guid[:])

		// invalid size
		err = guid.UnmarshalJSON(nil)
		require.Error(t, err)
	})

	t.Run("json.Unmarshal", func(t *testing.T) {
		testdata := struct {
			Data GUID `json:"data"`
		}{}
		const format = `{"data": "%s"}`
		jsonData := []byte(fmt.Sprintf(format, strings.Repeat("01", Size)))
		err := json.Unmarshal(jsonData, &testdata)
		require.NoError(t, err)
		expected := bytes.Repeat([]byte{1}, Size)
		require.Equal(t, expected, testdata.Data[:])

		jsonData, err = json.Marshal(testdata)
		require.NoError(t, err)
		fmt.Println(string(jsonData))
	})
}

func testPrintGUID(t testing.TB, guid *GUID) {
	t.Log(guid[:])
	t.Log(guid.Print())
	t.Log(guid.Hex())
	t.Log()
}

func TestGenerator(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("with no now function", func(t *testing.T) {
		g := New(16, nil)
		for i := 0; i < 4; i++ {
			testPrintGUID(t, g.Get())
		}
		g.Close()
		testsuite.IsDestroyed(t, g)
	})

	t.Run("with now()", func(t *testing.T) {
		g := New(16, time.Now)
		for i := 0; i < 4; i++ {
			testPrintGUID(t, g.Get())
		}
		g.Close()
		testsuite.IsDestroyed(t, g)
	})

	t.Run("zero size", func(t *testing.T) {
		g := New(0, time.Now)
		for i := 0; i < 4; i++ {
			testPrintGUID(t, g.Get())
		}
		g.Close()
		// twice
		g.Close()
		testsuite.IsDestroyed(t, g)
	})

	t.Run("panic in generator()", func(t *testing.T) {
		patchFunc := func(_ interface{}, _ []byte, _ uint32) {
			panic(monkey.Panic)
		}
		pg := monkey.PatchInstanceMethod(binary.BigEndian, "PutUint32", patchFunc)
		go func() {
			time.Sleep(time.Second)
			pg.Unpatch()
		}()
		g := New(0, time.Now)
		for i := 0; i < 4; i++ {
			testPrintGUID(t, g.Get())
		}
		g.Close()
		testsuite.IsDestroyed(t, g)
	})
}

func BenchmarkGenerator_Get(b *testing.B) {
	gm := testsuite.MarkGoroutines(b)
	defer gm.Compare()

	g := New(512, nil)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.Get()
	}
	b.StopTimer()
	g.Close()
	testsuite.IsDestroyed(b, g)
}

func BenchmarkGUIDWithMapKey(b *testing.B) {
	gm := testsuite.MarkGoroutines(b)
	defer gm.Compare()

	rand := random.New()
	key := make([]GUID, b.N)
	for i := 0; i < b.N; i++ {
		b := rand.Bytes(Size)
		copy(key[i][:], b)
	}
	m := make(map[GUID]int)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m[key[i]] = i
	}
	b.StopTimer()
}
