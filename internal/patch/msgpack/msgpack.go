package msgpack

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/vmihailenco/msgpack/v4"
)

// Encoder is a type alias.
type Encoder = msgpack.Encoder

// Decoder is a decoder, it will return error if find unknown field.
type Decoder struct {
	*msgpack.Decoder
}

// NewEncoder returns a new encoder that writes to w.
func NewEncoder(w io.Writer) *Encoder {
	encoder := msgpack.NewEncoder(w)
	encoder.UseCompactEncoding(true)
	encoder.UseCompactFloats(true)
	return encoder
}

// NewDecoder returns a new decoder that reads from r.
func NewDecoder(r io.Reader) *Decoder {
	decoder := msgpack.NewDecoder(r)
	decoder.DisallowUnknownFields()
	return &Decoder{Decoder: decoder}
}

// Decode reads the msgpack encoded data and stores it
// in the value pointed to by v.
func (dec *Decoder) Decode(v interface{}) error {
	err := dec.Decoder.Decode(v)
	if err == nil {
		return nil
	}
	errStr := err.Error()
	if strings.Contains(errStr, "unknown field") {
		name := reflect.TypeOf(v).String()
		return fmt.Errorf("%s in %s", errStr, name)
	}
	return err
}

// Marshal returns the MessagePack encoding of v.
func Marshal(v interface{}) ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0, 64))
	err := NewEncoder(buf).Encode(v)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Unmarshal decodes the MessagePack-encoded data and stores
// the result in the value pointed to by v.
func Unmarshal(data []byte, v interface{}) error {
	return NewDecoder(bytes.NewReader(data)).Decode(v)
}
