package vm

import (
	"errors"
	"testing"
)

func TestNewError(t *testing.T) {
	err := newError(nil, nil)
	if err != nil {
		t.Fatal("err is not nil")
	}
	err = newError(nil, errors.New("foo"))
	if err == nil {
		t.Fatal("err is nil")
	}
}

func TestNewStringError(t *testing.T) {
	err := newStringError(nil, "")
	if err != nil {
		t.Fatal("err is not nil")
	}
	err = newStringError(nil, "foo")
	if err == nil {
		t.Fatal("err is nil")
	}
}
