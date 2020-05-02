package testsuite

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/nettool"
	"project/internal/patch/monkey"
)

func TestPrintNetworkInfo(t *testing.T) {
	patch := func() (bool, bool) {
		return false, false
	}
	pg := monkey.Patch(nettool.IPEnabled, patch)
	defer pg.Unpatch()

	printNetworkInfo()
}

func TestDeployPPROFHTTPServer(t *testing.T) {
	defer func() {
		r := recover()
		require.NotNil(t, r)
		t.Log(r)
	}()
	patch := func(int) bool {
		return false
	}
	pg := monkey.Patch(startPPROFHTTPServer, patch)
	defer pg.Unpatch()

	deployPPROFHTTPServer()
}

func TestStartPPROFHTTPServer(t *testing.T) {
	t.Run("tcp4", func(t *testing.T) {
		patch := func(string, string) (net.Listener, error) {
			return nil, monkey.Error
		}
		pg := monkey.Patch(net.Listen, patch)
		defer pg.Unpatch()

		ok := startPPROFHTTPServer(123)
		require.False(t, ok)
	})

	t.Run("tcp6", func(t *testing.T) {
		patch := func(network, address string) (net.Listener, error) {
			if network == "tcp6" {
				return nil, monkey.Error
			}
			return nil, nil
		}
		pg := monkey.Patch(net.Listen, patch)
		defer pg.Unpatch()

		ok := startPPROFHTTPServer(123)
		require.False(t, ok)
	})
}

func TestIsInGoland(t *testing.T) {
	t.Log("in Goland:", InGoland)
}

func TestBytes(t *testing.T) {
	Bytes()
}

func TestIsDestroyed(t *testing.T) {
	a := 1
	n, err := fmt.Fprintln(ioutil.Discard, a)
	require.Equal(t, n, 2)
	require.NoError(t, err)
	if !Destroyed(&a) {
		t.Fatal("doesn't destroyed")
	}

	b := 2
	if Destroyed(&b) {
		t.Fatal("destroyed")
	}
	n, err = fmt.Fprintln(ioutil.Discard, b)
	require.Equal(t, n, 2)
	require.NoError(t, err)

	c := 3
	n, err = fmt.Fprintln(ioutil.Discard, c)
	require.Equal(t, n, 2)
	require.NoError(t, err)
	IsDestroyed(t, &c)
}

func TestCheckErrorInTestMain(t *testing.T) {
	defer func() {
		r := recover()
		require.NotNil(t, r)
		t.Log(r)
	}()
	CheckErrorInTestMain(errors.New("foo error"))
}

type testOptions struct {
	SF testOptionsB `check:"-"`

	Foo int
	Bar string
	BA  testOptionsB
	BB  *testOptionsB

	SA testOptionsB  `check:"-"`
	SB *testOptionsB `check:"-"`
	SC string        `check:"-"`
}

type testOptionsB struct {
	SF int `check:"-"`

	A int
	B string
	C *testOptionsC

	SA int `check:"-"`
}

type testOptionsC struct {
	SF int `check:"-"`

	D int

	SA int `check:"-"`
}

type testOptionNest struct {
	A int
	B struct {
		NA int
		NB string
	}
}

func TestCheckOptions(t *testing.T) {
	ob := testOptionsB{
		A: 123,
		B: "bbb",
		C: &testOptionsC{D: 123},
	}
	t.Run("ok", func(t *testing.T) {
		opts := testOptions{
			Foo: 123,
			Bar: "bar",
			BA:  ob,
			BB:  &ob,
		}
		CheckOptions(t, opts)
		CheckOptions(t, &opts)
	})

	t.Run("foo", func(t *testing.T) {
		const except = "testOptions.Foo is zero value"
		opts := testOptions{
			Bar: "",
			BA:  ob,
			BB:  &ob,
		}
		require.Equal(t, except, checkOptions("", opts))
	})

	t.Run("bar", func(t *testing.T) {
		const except = "testOptions.Bar is zero value"
		opts := testOptions{
			Foo: 123,
			BA:  ob,
			BB:  &ob,
		}
		require.Equal(t, except, checkOptions("", opts))
	})

	t.Run("BA.A", func(t *testing.T) {
		const except = "testOptions.BA.A is zero value"
		opts := testOptions{
			Foo: 123,
			Bar: "bar",
		}
		require.Equal(t, except, checkOptions("", opts))
	})

	t.Run("BA.B", func(t *testing.T) {
		const except = "testOptions.BA.B is zero value"
		opts := testOptions{
			Foo: 123,
			Bar: "bar",
		}
		opts.BA.A = 123
		require.Equal(t, except, checkOptions("", opts))
	})

	t.Run("BA.C-nil point", func(t *testing.T) {
		const except = "testOptions.BA.C is nil point"
		opts := testOptions{
			Foo: 123,
			Bar: "bar",
		}
		opts.BA.A = 123
		opts.BA.B = "bar"
		require.Equal(t, except, checkOptions("", opts))
	})

	t.Run("BB-nil point", func(t *testing.T) {
		const except = "testOptions.BB is nil point"
		opts := testOptions{
			Foo: 123,
			Bar: "bar",
			BA: testOptionsB{
				A: 123,
				B: "bbb",
				C: &testOptionsC{
					D: 123,
				},
			},
		}
		require.Equal(t, except, checkOptions("", opts))
	})

	t.Run("BB.A", func(t *testing.T) {
		const except = "testOptions.BB.A is zero value"
		opts := testOptions{
			Foo: 123,
			Bar: "bar",
			BA: testOptionsB{
				A: 123,
				B: "bbb",
				C: &testOptionsC{
					D: 123,
				},
			},
			BB: &testOptionsB{},
		}
		require.Equal(t, except, checkOptions("", opts))
	})

	t.Run("BB.B", func(t *testing.T) {
		const except = "testOptions.BB.B is zero value"
		opts := testOptions{
			Foo: 123,
			Bar: "bar",
			BA: testOptionsB{
				A: 123,
				B: "bbb",
				C: &testOptionsC{
					D: 123,
				},
			},
			BB: &testOptionsB{
				A: 123,
			},
		}
		require.Equal(t, except, checkOptions("", opts))
	})

	t.Run("BB.C-nil point", func(t *testing.T) {
		const except = "testOptions.BB.C is nil point"
		opts := testOptions{
			Foo: 123,
			Bar: "bar",
			BA: testOptionsB{
				A: 123,
				B: "bbb",
				C: &testOptionsC{
					D: 123,
				},
			},
			BB: &testOptionsB{
				A: 123,
				B: "bbb",
			},
		}
		require.Equal(t, except, checkOptions("", opts))
	})

	t.Run("BB.C.D", func(t *testing.T) {
		const except = "testOptions.BB.C.D is zero value"
		opts := testOptions{
			Foo: 123,
			Bar: "bar",
			BA: testOptionsB{
				A: 123,
				B: "bbb",
				C: &testOptionsC{
					D: 123,
				},
			},
			BB: &testOptionsB{
				A: 123,
				B: "bbb",
				C: &testOptionsC{},
			},
		}
		require.Equal(t, except, checkOptions("", opts))
	})

	t.Run("nest-ok", func(t *testing.T) {
		opts := testOptionNest{
			A: 1,
			B: struct {
				NA int
				NB string
			}{
				NA: 123,
				NB: "nb",
			},
		}
		CheckOptions(t, &opts)
	})

	t.Run("nest-B.NA", func(t *testing.T) {
		const except = "testOptionNest.B.NA is zero value"
		opts := testOptionNest{
			A: 1,
			B: struct {
				NA int
				NB string
			}{},
		}
		require.Equal(t, except, checkOptions("", opts))
	})

	t.Run("nest-B.NB", func(t *testing.T) {
		const except = "testOptionNest.B.NB is zero value"
		opts := testOptionNest{
			A: 1,
			B: struct {
				NA int
				NB string
			}{
				NA: 123,
			},
		}
		require.Equal(t, except, checkOptions("", opts))
	})
}

func TestRunParallel(t *testing.T) {
	gm := MarkGoroutines(t)
	defer gm.Compare()

	test := 0
	m := sync.Mutex{}

	f1 := func() {
		m.Lock()
		defer m.Unlock()

		test++
		fmt.Println(test)
	}
	f2 := func() {
		m.Lock()
		defer m.Unlock()

		test++
		fmt.Println(test)
	}

	RunParallel(f1, f2)

	// no functions
	RunParallel()
}

func TestHTTPServer(t *testing.T) {
	gm := MarkGoroutines(t)
	defer gm.Compare()

	// http
	httpServer := http.Server{Addr: "localhost:0"}
	port := RunHTTPServer(t, "tcp", &httpServer)
	defer func() { _ = httpServer.Close() }()
	t.Log("http server port:", port)
	client := http.Client{}
	resp, err := client.Get(fmt.Sprintf("http://localhost:%s/", port))
	require.NoError(t, err)
	_, err = io.Copy(ioutil.Discard, resp.Body)
	require.NoError(t, err)
	client.CloseIdleConnections()

	// https
	serverCfg, clientCfg := TLSConfigPair(t)
	httpsServer := http.Server{
		Addr:      "localhost:0",
		TLSConfig: serverCfg,
	}
	port = RunHTTPServer(t, "tcp", &httpsServer)
	defer func() { _ = httpsServer.Close() }()
	t.Log("https server port:", port)
	client = http.Client{
		Transport: &http.Transport{
			TLSClientConfig: clientCfg,
		},
	}
	resp, err = client.Get(fmt.Sprintf("https://localhost:%s/", port))
	require.NoError(t, err)
	_, err = io.Copy(ioutil.Discard, resp.Body)
	require.NoError(t, err)
	client.CloseIdleConnections()
}
