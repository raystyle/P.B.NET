package testsuite

import (
	"net"
	"testing"
	"time"
	"unsafe"

	"github.com/stretchr/testify/require"
)

type testOptions struct {
	SF testOptionsB `testsuite:"-"`

	Foo int
	Bar string
	BA  testOptionsB
	BB  *testOptionsB

	Skip1 chan string
	Skip2 func()
	Skip3 net.Conn
	Skip4 complex64
	Skip5 complex128
	Skip6 unsafe.Pointer

	unexported int

	SA testOptionsB  `testsuite:"-"`
	SB *testOptionsB `testsuite:"-"`
	SC string        `testsuite:"-"`
}

type testOptionsB struct {
	SF int `testsuite:"-"`

	A int
	B string
	C *testOptionsC

	SA int `testsuite:"-"`
}

type testOptionsC struct {
	SF int `testsuite:"-"`

	D int

	SA int `testsuite:"-"`
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

func TestContainZeroValue(t *testing.T) {
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
		ContainZeroValue(t, opts)
		ContainZeroValue(t, &opts)
	})

	t.Run("foo", func(t *testing.T) {
		const expected = "testOptions.Foo is zero value"
		opts := testOptions{
			Bar: "",
			BA:  ob,
			BB:  &ob,
		}
		require.Equal(t, expected, containZeroValue("", opts))
	})

	t.Run("bar", func(t *testing.T) {
		const expected = "testOptions.Bar is zero value"
		opts := testOptions{
			Foo: 123,
			BA:  ob,
			BB:  &ob,
		}
		require.Equal(t, expected, containZeroValue("", opts))
	})

	t.Run("BA.A", func(t *testing.T) {
		const expected = "testOptions.BA.A is zero value"
		opts := testOptions{
			Foo: 123,
			Bar: "bar",
		}
		require.Equal(t, expected, containZeroValue("", opts))
	})

	t.Run("BA.B", func(t *testing.T) {
		const expected = "testOptions.BA.B is zero value"
		opts := testOptions{
			Foo: 123,
			Bar: "bar",
		}
		opts.BA.A = 123
		require.Equal(t, expected, containZeroValue("", opts))
	})

	t.Run("BA.C-nil point", func(t *testing.T) {
		const expected = "testOptions.BA.C is nil point"
		opts := testOptions{
			Foo: 123,
			Bar: "bar",
		}
		opts.BA.A = 123
		opts.BA.B = "bar"
		require.Equal(t, expected, containZeroValue("", opts))
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
		require.Equal(t, expected, containZeroValue("", opts))
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
		require.Equal(t, expected, containZeroValue("", opts))
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
		require.Equal(t, expected, containZeroValue("", opts))
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
		require.Equal(t, expected, containZeroValue("", opts))
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
		require.Equal(t, expected, containZeroValue("", opts))
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
		ContainZeroValue(t, &opts)
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
		require.Equal(t, expected, containZeroValue("", opts))
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
		require.Equal(t, expected, containZeroValue("", opts))
	})

	t.Run("skip time.Time", func(t *testing.T) {
		t.Run("single", func(t *testing.T) {
			const expected = "time.Time is zero value"
			ti := time.Time{}
			require.Equal(t, expected, containZeroValue("", ti))
			require.Equal(t, expected, containZeroValue("", &ti))
		})

		t.Run("struct", func(t *testing.T) {
			ti := time.Time{}.AddDate(2017, 10, 26) // 2018-11-27

			opts := testOptionSpecial{
				A: "a",
				B: ti,
				C: &ti,
			}
			require.Zero(t, containZeroValue("", opts))

			const (
				expected1 = "testOptionSpecial.B is zero value"
				expected2 = "testOptionSpecial.C is zero value"
			)

			opts.B = time.Time{}
			require.Equal(t, expected1, containZeroValue("", opts))
			opts.B = ti

			opts.C = nil
			require.Equal(t, expected2, containZeroValue("", opts))
		})
	})

	t.Run("empty structure tag value", func(t *testing.T) {
		opts := struct {
			A string `testsuite:""`
		}{}
		result := containZeroValue("", opts)
		require.NotZero(t, result)
		t.Log("result:", result)
	})
}
