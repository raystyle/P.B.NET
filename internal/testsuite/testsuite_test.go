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
	"time"
	"unsafe"

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

func TestDeployPprofHTTPServer(t *testing.T) {
	defer DeferForPanic(t)

	patch := func(string, string) (net.Listener, error) {
		return nil, monkey.Error
	}
	pg := monkey.Patch(net.Listen, patch)
	defer pg.Unpatch()

	deployPprofHTTPServer()
}

func TestStartPprofHTTPServer(t *testing.T) {
	t.Run("tcp4", func(t *testing.T) {
		patch := func(string, string) (net.Listener, error) {
			return nil, monkey.Error
		}
		pg := monkey.Patch(net.Listen, patch)
		defer pg.Unpatch()

		ok := startPprofHTTPServer(nil, 123)
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

		ok := startPprofHTTPServer(nil, 123)
		require.False(t, ok)
	})
}

func TestIsInGoland(t *testing.T) {
	t.Log("in Goland:", InGoland)
}

func TestBytes(t *testing.T) {
	Bytes()
}

func TestDeferForPanic(t *testing.T) {
	defer DeferForPanic(t)

	panic("test panic")
}

func TestCheckErrorInTestMain(t *testing.T) {
	defer DeferForPanic(t)

	CheckErrorInTestMain(errors.New("foo error"))
}

type testOptions struct {
	SF testOptionsB `check:"-"`

	Foo int
	Bar string
	BA  testOptionsB
	BB  *testOptionsB

	Skip1 func()
	Skip2 chan string
	Skip3 complex64
	Skip4 complex128
	Skip5 unsafe.Pointer

	unexported int

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

type testOptionSpecial struct {
	A string
	B time.Time
	C *time.Time
}

func TestCheckOptions(t *testing.T) {
	ob := testOptionsB{
		A: 123,
		B: "bbb",
		C: &testOptionsC{D: 123},
	}
	t.Run("ok", func(t *testing.T) {
		opts := testOptions{
			Foo:        123,
			Bar:        "bar",
			BA:         ob,
			BB:         &ob,
			unexported: 0,
		}
		CheckOptions(t, opts)
		CheckOptions(t, &opts)
	})

	t.Run("foo", func(t *testing.T) {
		const expected = "testOptions.Foo is zero value"
		opts := testOptions{
			Bar: "",
			BA:  ob,
			BB:  &ob,
		}
		require.Equal(t, expected, checkOptions("", opts))
	})

	t.Run("bar", func(t *testing.T) {
		const expected = "testOptions.Bar is zero value"
		opts := testOptions{
			Foo: 123,
			BA:  ob,
			BB:  &ob,
		}
		require.Equal(t, expected, checkOptions("", opts))
	})

	t.Run("BA.A", func(t *testing.T) {
		const expected = "testOptions.BA.A is zero value"
		opts := testOptions{
			Foo: 123,
			Bar: "bar",
		}
		require.Equal(t, expected, checkOptions("", opts))
	})

	t.Run("BA.B", func(t *testing.T) {
		const expected = "testOptions.BA.B is zero value"
		opts := testOptions{
			Foo: 123,
			Bar: "bar",
		}
		opts.BA.A = 123
		require.Equal(t, expected, checkOptions("", opts))
	})

	t.Run("BA.C-nil point", func(t *testing.T) {
		const expected = "testOptions.BA.C is nil point"
		opts := testOptions{
			Foo: 123,
			Bar: "bar",
		}
		opts.BA.A = 123
		opts.BA.B = "bar"
		require.Equal(t, expected, checkOptions("", opts))
	})

	t.Run("BB-nil point", func(t *testing.T) {
		const expected = "testOptions.BB is nil point"
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
		require.Equal(t, expected, checkOptions("", opts))
	})

	t.Run("BB.A", func(t *testing.T) {
		const expected = "testOptions.BB.A is zero value"
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
		require.Equal(t, expected, checkOptions("", opts))
	})

	t.Run("BB.B", func(t *testing.T) {
		const expected = "testOptions.BB.B is zero value"
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
		require.Equal(t, expected, checkOptions("", opts))
	})

	t.Run("BB.C-nil point", func(t *testing.T) {
		const expected = "testOptions.BB.C is nil point"
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
		require.Equal(t, expected, checkOptions("", opts))
	})

	t.Run("BB.C.D", func(t *testing.T) {
		const expected = "testOptions.BB.C.D is zero value"
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
		require.Equal(t, expected, checkOptions("", opts))
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
		const expected = "testOptionNest.B.NA is zero value"
		opts := testOptionNest{
			A: 1,
			B: struct {
				NA int
				NB string
			}{},
		}
		require.Equal(t, expected, checkOptions("", opts))
	})

	t.Run("nest-B.NB", func(t *testing.T) {
		const expected = "testOptionNest.B.NB is zero value"
		opts := testOptionNest{
			A: 1,
			B: struct {
				NA int
				NB string
			}{
				NA: 123,
			},
		}
		require.Equal(t, expected, checkOptions("", opts))
	})

	t.Run("skip time.Time", func(t *testing.T) {
		t.Run("single", func(t *testing.T) {
			const expected = "time.Time is zero value"
			ti := time.Time{}
			require.Equal(t, expected, checkOptions("", ti))
			require.Equal(t, expected, checkOptions("", &ti))
		})

		t.Run("struct", func(t *testing.T) {
			ti := time.Time{}.AddDate(2017, 10, 26) // 2018-11-27

			opts := testOptionSpecial{
				A: "a",
				B: ti,
				C: &ti,
			}
			require.Zero(t, checkOptions("", opts))

			const (
				expected1 = "testOptionSpecial.B is zero value"
				expected2 = "testOptionSpecial.C is zero value"
			)

			opts.B = time.Time{}
			require.Equal(t, expected1, checkOptions("", opts))
			opts.B = ti

			opts.C = nil
			require.Equal(t, expected2, checkOptions("", opts))
		})
	})

	t.Run("appear panic", func(t *testing.T) {
		opts := struct {
			P io.Writer
		}{}
		result := checkOptions("", opts)
		require.NotZero(t, result)
		t.Log("result:", result)
	})
}

func TestRunMultiTimes(t *testing.T) {
	gm := MarkGoroutines(t)
	defer gm.Compare()

	test := 0
	m := sync.Mutex{}

	f1 := func() {
		m.Lock()
		defer m.Unlock()

		test++
		fmt.Println("f1:", test)
	}
	f2 := func() {
		m.Lock()
		defer m.Unlock()

		test += 2
		fmt.Println("f2:", test)
	}
	f3 := func() {
		m.Lock()
		defer m.Unlock()

		test += 3
		fmt.Println("f3:", test)
	}

	t.Run("ok", func(t *testing.T) {
		RunMultiTimes(5, f1, f2, f3)

		require.Equal(t, (1+2+3)*5, test)
	})

	t.Run("no functions", func(t *testing.T) {
		RunMultiTimes(1)
	})

	t.Run("invalid times", func(t *testing.T) {
		test = 0

		RunMultiTimes(-1, f3)

		require.Equal(t, 3*100, test)
	})
}

func TestRunParallel(t *testing.T) {
	gm := MarkGoroutines(t)
	defer gm.Compare()

	test := 0
	m := sync.Mutex{}

	init := func() {
		test = 0
		fmt.Println("init")
	}
	f1 := func() {
		m.Lock()
		defer m.Unlock()

		test++
		fmt.Println("f1:", test)
	}
	f2 := func() {
		m.Lock()
		defer m.Unlock()

		test += 2
		fmt.Println("f2:", test)
	}
	f3 := func() {
		m.Lock()
		defer m.Unlock()

		test += 3
		fmt.Println("f3:", test)
	}
	cleanup := func() {
		fmt.Println("cleanup")
	}

	t.Run("ok", func(t *testing.T) {
		RunParallel(5, init, cleanup, f1, f2, f3)

		require.Equal(t, 1+2+3, test)
	})

	t.Run("no functions", func(t *testing.T) {
		RunParallel(1, nil, nil)
	})

	t.Run("invalid times", func(t *testing.T) {
		test = 0

		RunParallel(-1, nil, nil, f3)

		require.Equal(t, 3*100, test)
	})
}

func TestRunHTTPServer(t *testing.T) {
	gm := MarkGoroutines(t)
	defer gm.Compare()

	t.Run("http", func(t *testing.T) {
		server := http.Server{Addr: "localhost:0"}
		port := RunHTTPServer(t, "tcp", &server)
		defer func() { _ = server.Close() }()
		t.Log("http server port:", port)

		client := http.Client{}
		defer client.CloseIdleConnections()
		resp, err := client.Get(fmt.Sprintf("http://localhost:%s/", port))
		require.NoError(t, err)
		_, err = io.Copy(ioutil.Discard, resp.Body)
		require.NoError(t, err)
	})

	t.Run("https", func(t *testing.T) {
		serverCfg, clientCfg := TLSConfigPair(t, "127.0.0.1")
		server := http.Server{
			Addr:      "localhost:0",
			TLSConfig: serverCfg,
		}
		port := RunHTTPServer(t, "tcp", &server)
		defer func() { _ = server.Close() }()
		t.Log("https server port:", port)

		client := http.Client{
			Transport: &http.Transport{
				TLSClientConfig: clientCfg,
			},
		}
		defer client.CloseIdleConnections()
		resp, err := client.Get(fmt.Sprintf("https://localhost:%s/", port))
		require.NoError(t, err)
		_, err = io.Copy(ioutil.Discard, resp.Body)
		require.NoError(t, err)
	})
}
