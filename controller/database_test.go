package controller

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_connect_database(t *testing.T) {
	_, err := connect_database(test_gen_config())
	require.Nil(t, err, err)
}

func Test_init_database(t *testing.T) {
	db, err := connect_database(test_gen_config())
	require.Nil(t, err, err)
	err = init_database(db)
	require.Nil(t, err, err)
}
