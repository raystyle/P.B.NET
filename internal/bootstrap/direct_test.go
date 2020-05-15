package bootstrap

import (
	"bytes"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/crypto/aes"
	"project/internal/testsuite"
)

func testGenerateDirect() *Direct {
	direct := NewDirect()
	direct.Listeners = testGenerateListeners()
	return direct
}

func TestDirect_Validate(t *testing.T) {
	direct := testGenerateDirect()

	err := direct.Validate()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, direct)
}

func TestDirect_Marshal(t *testing.T) {
	direct := testGenerateDirect()

	t.Run("ok", func(t *testing.T) {
		data, err := direct.Marshal()
		require.NoError(t, err)

		t.Log(string(data))
	})

	t.Run("failed", func(t *testing.T) {
		direct.Listeners = nil

		data, err := direct.Marshal()
		require.Error(t, err)
		require.Nil(t, data)
	})

	testsuite.IsDestroyed(t, direct)
}

func TestDirect_Unmarshal(t *testing.T) {
	direct := NewDirect()

	t.Run("ok", func(t *testing.T) {
		err := direct.Unmarshal(nil)
		require.NoError(t, err)
	})

	t.Run("failed", func(t *testing.T) {
		err := direct.Unmarshal([]byte{0x00})
		require.Error(t, err)
	})

	testsuite.IsDestroyed(t, direct)
}

func TestDirect_Resolve(t *testing.T) {
	listeners := testGenerateListeners()

	direct := NewDirect()
	direct.Listeners = listeners

	data, err := direct.Marshal()
	require.NoError(t, err)
	direct = NewDirect()
	err = direct.Unmarshal(data)
	require.NoError(t, err)

	for i := 0; i < 10; i++ {
		resolved, err := direct.Resolve()
		require.NoError(t, err)
		resolved = testDecryptListeners(resolved)
		require.Equal(t, listeners, resolved)
	}

	testsuite.IsDestroyed(t, direct)
}

func TestDirectPanic(t *testing.T) {
	t.Run("no CBC", func(t *testing.T) {
		direct := NewDirect()

		func() {
			defer testsuite.DeferForPanic(t)
			_, _ = direct.Resolve()
		}()

		testsuite.IsDestroyed(t, direct)
	})

	t.Run("invalid node listeners data", func(t *testing.T) {
		direct := NewDirect()

		func() {
			var err error
			key := bytes.Repeat([]byte{0}, aes.Key128Bit)
			direct.cbc, err = aes.NewCBC(key, key)
			require.NoError(t, err)
			direct.enc, err = direct.cbc.Encrypt(testsuite.Bytes())
			require.NoError(t, err)

			defer testsuite.DeferForPanic(t)
			_, _ = direct.Resolve()
		}()

		testsuite.IsDestroyed(t, direct)
	})
}

func TestDirectOptions(t *testing.T) {
	config, err := ioutil.ReadFile("testdata/direct.toml")
	require.NoError(t, err)

	// check unnecessary field
	direct := NewDirect()
	err = direct.Unmarshal(config)
	require.NoError(t, err)
	err = direct.Validate()
	require.NoError(t, err)

	// check zero value
	testsuite.CheckOptions(t, direct)

	listeners := testGenerateListeners()
	for i := 0; i < 10; i++ {
		resolved, err := direct.Resolve()
		require.NoError(t, err)
		resolved = testDecryptListeners(resolved)
		require.Equal(t, listeners, resolved)
	}
}
