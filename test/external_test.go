package test

import (
	"testing"

	"github.com/pelletier/go-toml"
	"github.com/stretchr/testify/require"
)

func TestToml_Negative(t *testing.T) {
	cfg := struct {
		Num uint
	}{}
	err := toml.Unmarshal([]byte(`Num = -1`), &cfg)
	require.Error(t, err)
	require.Equal(t, uint(0), cfg.Num)
}
