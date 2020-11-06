package main

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/patch/toml"
	"project/internal/testsuite"
)

func TestConfig(t *testing.T) {
	data, err := ioutil.ReadFile("testdata/config.toml")
	require.NoError(t, err)

	// check unnecessary field
	var config config
	err = toml.Unmarshal(data, &config)
	require.NoError(t, err)

	// check zero value
	testsuite.ContainZeroValue(t, config)
}
