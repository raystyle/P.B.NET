package httptime

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Query(t *testing.T) {
	r, err := http.NewRequest(http.MethodGet, "https://www.baidu.com", nil)
	require.Nil(t, err, err)
	_time, err := Query(r, nil)
	require.Nil(t, err, err)
	t.Log(_time)
	// custom client
	client := &http.Client{}
	_time, err = Query(r, client)
	require.Nil(t, err, err)
	t.Log(_time)
	// query failed
	r2, err := http.NewRequest(http.MethodGet, "http://asdasd1516ads.com", nil)
	require.Nil(t, err, err)
	_, err = Query(r2, client)
	require.NotNil(t, err)
}
