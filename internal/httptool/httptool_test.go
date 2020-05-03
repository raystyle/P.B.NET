package httptool

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/patch/monkey"
)

func testGenerateRequest(t *testing.T) *http.Request {
	r, err := http.NewRequest(http.MethodGet, "https://github.com/", nil)
	require.NoError(t, err)
	r.RemoteAddr = "127.0.0.1:1234"
	r.RequestURI = "/index"
	r.Header.Set("User-Agent", "Mozilla")
	r.Header.Set("Accept", "text/html")
	r.Header.Set("Connection", "keep-alive")
	return r
}

func TestFprintRequest(t *testing.T) {
	r := testGenerateRequest(t)

	fmt.Println("-----begin (GET and no body)-----")
	fmt.Println(PrintRequest(r))
	fmt.Printf("-----end-----\n\n")

	equalBody := func(b1, b2 io.Reader) {
		d1, err := ioutil.ReadAll(b1)
		require.NoError(t, err)
		d2, err := ioutil.ReadAll(b2)
		require.NoError(t, err)
		require.Equal(t, d1, d2)
	}

	body := new(bytes.Buffer)
	rawBody := bytes.NewReader(body.Bytes())
	r.Body = ioutil.NopCloser(body)
	fmt.Println("-----begin (GET with body but no data)-----")
	fmt.Println(PrintRequest(r))
	fmt.Printf("-----end-----\n\n")
	equalBody(rawBody, r.Body)

	body.Reset()
	body.WriteString(strings.Repeat("a", bodyLineLength-10))
	rawBody = bytes.NewReader(body.Bytes())
	r.Body = ioutil.NopCloser(body)
	fmt.Println("-----begin (POST with data <bodyLineLength)-----")
	fmt.Println(PrintRequest(r))
	fmt.Printf("-----end-----\n\n")
	equalBody(rawBody, r.Body)

	body.Reset()
	body.WriteString(strings.Repeat("a", bodyLineLength))
	rawBody = bytes.NewReader(body.Bytes())
	r.Body = ioutil.NopCloser(body)
	fmt.Println("-----begin (POST with data bodyLineLength)-----")
	fmt.Println(PrintRequest(r))
	fmt.Printf("-----end-----\n\n")
	equalBody(rawBody, r.Body)

	body.Reset()
	body.WriteString(strings.Repeat("a", 3*bodyLineLength-1))
	rawBody = bytes.NewReader(body.Bytes())
	r.Body = ioutil.NopCloser(body)
	fmt.Println("-----begin (POST with data 3*bodyLineLength-1)-----")
	fmt.Println(PrintRequest(r))
	fmt.Printf("-----end-----\n\n")
	equalBody(rawBody, r.Body)

	body.Reset()
	body.WriteString(strings.Repeat("a", 100*bodyLineLength-1))
	rawBody = bytes.NewReader(body.Bytes())
	r.Body = ioutil.NopCloser(body)
	fmt.Println("-----begin (POST with data 100*bodyLineLength-1)-----")
	fmt.Println(PrintRequest(r))
	fmt.Printf("-----end-----\n\n")
	equalBody(rawBody, r.Body)
}

func TestFprintRequestWithError(t *testing.T) {
	r := testGenerateRequest(t)

	for _, test := range []struct {
		name   string
		format string
	}{
		{"client", "client: %s\n"},
		{"request", "%s %s %s"},
		{"host", "\nHost: %s"},
		{"header", "\n%s: %s"},
	} {
		t.Run(test.name, func(t *testing.T) {
			patch := func(w io.Writer, format string, a ...interface{}) (int, error) {
				if format == test.format {
					return 0, monkey.Error
				}
				return w.Write([]byte(fmt.Sprintf(format, a...)))
			}
			pg := monkey.Patch(fmt.Fprintf, patch)
			defer pg.Unpatch()
			_, err := FprintRequest(os.Stdout, r)
			monkey.IsMonkeyError(t, err)

			// fix goland new line bug
			fmt.Println()
		})
	}
}

func TestPrintBody(t *testing.T) {
	r := testGenerateRequest(t)

	t.Run("size < bodyLineLength", func(t *testing.T) {
		patch := func(w io.Writer, format string, a ...interface{}) (int, error) {
			if format == "\n\n%s" {
				return 0, monkey.Error
			}
			return w.Write([]byte(fmt.Sprintf(format, a...)))
		}
		pg := monkey.Patch(fmt.Fprintf, patch)
		defer pg.Unpatch()

		r.Body = ioutil.NopCloser(strings.NewReader("test"))
		_, err := FprintRequest(os.Stdout, r)
		monkey.IsMonkeyError(t, err)

		// fix goland new line bug
		fmt.Println()
	})

	t.Run("bodyLineLength < size < 2x", func(t *testing.T) {
		patch := func(w io.Writer, format string, a ...interface{}) (int, error) {
			if format == "\n\n%s" {
				return 0, monkey.Error
			}
			return w.Write([]byte(fmt.Sprintf(format, a...)))
		}
		pg := monkey.Patch(fmt.Fprintf, patch)
		defer pg.Unpatch()

		testdata := "test" + strings.Repeat("a", bodyLineLength)
		r.Body = ioutil.NopCloser(strings.NewReader(testdata))
		_, err := FprintRequest(os.Stdout, r)
		monkey.IsMonkeyError(t, err)

		// fix goland new line bug
		fmt.Println()
	})

	t.Run("1.5 x bodyLineLength", func(t *testing.T) {
		patch := func(w io.Writer, format string, a ...interface{}) (int, error) {
			if format == "\n%s" {
				return 0, monkey.Error
			}
			return w.Write([]byte(fmt.Sprintf(format, a...)))
		}
		pg := monkey.Patch(fmt.Fprintf, patch)
		defer pg.Unpatch()

		testdata := "test" + strings.Repeat("a", bodyLineLength)
		r.Body = ioutil.NopCloser(strings.NewReader(testdata))
		_, err := FprintRequest(os.Stdout, r)
		monkey.IsMonkeyError(t, err)

		// fix goland new line bug
		fmt.Println()
	})

	t.Run("2.5 x bodyLineLength", func(t *testing.T) {
		patch := func(w io.Writer, format string, a ...interface{}) (int, error) {
			if format == "\n%s" {
				return 0, monkey.Error
			}
			return w.Write([]byte(fmt.Sprintf(format, a...)))
		}
		pg := monkey.Patch(fmt.Fprintf, patch)
		defer pg.Unpatch()

		testdata := "test" + strings.Repeat("a", 2*bodyLineLength)
		r.Body = ioutil.NopCloser(strings.NewReader(testdata))
		_, err := FprintRequest(os.Stdout, r)
		monkey.IsMonkeyError(t, err)

		// fix goland new line bug
		fmt.Println()
	})
}

func TestSubHTTPFileSystem_Open(t *testing.T) {
	wd, err := os.Getwd()
	require.NoError(t, err)
	fs := http.Dir(wd)

	sfs := NewSubHTTPFileSystem(fs, "testdata")
	file, err := sfs.Open("data.txt")
	require.NoError(t, err)
	data, err := ioutil.ReadAll(file)
	require.NoError(t, err)
	require.Equal(t, "hello", string(data))

	file, err = sfs.Open("foo")
	require.Error(t, err)
	require.Nil(t, file)
}
