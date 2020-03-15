package controller

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/guid"
	"project/internal/module/info"
	"project/internal/patch/json"
)

func TestHexByteSlice(t *testing.T) {
	test := struct {
		Data hexByteSlice
	}{}
	test.Data = []byte("hello")

	data, err := json.Marshal(test)
	require.NoError(t, err)
	fmt.Println(string(data))

	test.Data = nil
	err = json.Unmarshal(data, &test)
	require.NoError(t, err)
	require.Equal(t, "hello", string(test.Data))

	t.Run("invalid size", func(t *testing.T) {
		err = test.Data.UnmarshalJSON(nil)
		require.Error(t, err)
	})

	t.Run("invalid hex data", func(t *testing.T) {
		err = json.Unmarshal([]byte(`{"Data":"foo data"}`), &test)
		require.Error(t, err)
	})
}

func testGenerateGUID() *guid.GUID {
	g := guid.GUID{}
	copy(g[:], bytes.Repeat([]byte{1}, guid.Size))
	return &g
}

func TestPrintActions(t *testing.T) {
	encoder := json.NewEncoder(64)

	t.Run("NoticeNodeRegister", func(t *testing.T) {
		nnr := NoticeNodeRegister{
			ID:           "id-01",
			GUID:         *testGenerateGUID(),
			PublicKey:    hexByteSlice(bytes.Repeat([]byte{2}, guid.Size)),
			KexPublicKey: hexByteSlice(bytes.Repeat([]byte{3}, guid.Size)),
			ConnAddress:  "127.0.0.1:9091",
			SystemInfo:   info.GetSystemInfo(),
			RequestTime:  time.Now(),
		}
		data, err := encoder.Encode(nnr)
		require.NoError(t, err)
		fmt.Println(string(data))
	})

	t.Run("NoticeBeaconRegister", func(t *testing.T) {
		nbr := NoticeBeaconRegister{
			ID:           "id-02",
			GUID:         *testGenerateGUID(),
			PublicKey:    hexByteSlice(bytes.Repeat([]byte{5}, guid.Size)),
			KexPublicKey: hexByteSlice(bytes.Repeat([]byte{6}, guid.Size)),
			ConnAddress:  "127.0.0.1:9092",
			SystemInfo:   info.GetSystemInfo(),
			RequestTime:  time.Now(),
		}
		data, err := encoder.Encode(nbr)
		require.NoError(t, err)
		fmt.Println(string(data))
	})
}
