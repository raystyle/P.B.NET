package bootstrap

import (
	"bytes"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/crypto/aes"
	"project/internal/testsuite"
)

func TestDirect(t *testing.T) {
	listeners := testGenerateListeners()
	direct := NewDirect()
	direct.Listeners = listeners
	_ = direct.Validate()
	b, err := direct.Marshal()
	require.NoError(t, err)
	testsuite.IsDestroyed(t, direct)

	direct = NewDirect()
	err = direct.Unmarshal(b)
	require.NoError(t, err)
	for i := 0; i < 10; i++ {
		resolved, _ := direct.Resolve()
		require.Equal(t, listeners, resolved)
	}
	testsuite.IsDestroyed(t, direct)
}

func TestDirect_Unmarshal(t *testing.T) {
	direct := NewDirect()
	b, err := direct.Marshal()
	require.Error(t, err)
	require.Nil(t, b)
	require.Error(t, direct.Unmarshal([]byte{0x00}))
	testsuite.IsDestroyed(t, direct)
}

func TestDirectPanic(t *testing.T) {
	t.Run("no CBC", func(t *testing.T) {
		direct := NewDirect()

		func() {
			defer func() {
				r := recover()
				require.NotNil(t, r)
				t.Log(r)
			}()
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
			enc, err := direct.cbc.Encrypt(testsuite.Bytes())
			require.NoError(t, err)
			direct.enc = enc

			defer func() {
				r := recover()
				require.NotNil(t, r)
				t.Log(r)
			}()
			_, _ = direct.Resolve()
		}()

		testsuite.IsDestroyed(t, direct)
	})
}

func TestDirectOptions(t *testing.T) {
	config, err := ioutil.ReadFile("testdata/direct.toml")
	require.NoError(t, err)
	direct := NewDirect()
	require.NoError(t, direct.Unmarshal(config))
	listeners := testGenerateListeners()
	for i := 0; i < 10; i++ {
		resolved, err := direct.Resolve()
		require.NoError(t, err)
		require.Equal(t, listeners, resolved)
	}
}
