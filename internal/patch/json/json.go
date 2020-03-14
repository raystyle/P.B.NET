package json

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
)

// Encoder is a encoder with buffer.
type Encoder struct {
	buf *bytes.Buffer
	*json.Encoder
}

// Decoder is a decoder, it will return error if find unknown field.
type Decoder struct {
	*json.Decoder
}

// NewEncoder returns a new encoder that writes to w.
func NewEncoder(size int) *Encoder {
	buf := bytes.NewBuffer(make([]byte, 0, size))
	encoder := json.NewEncoder(buf)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(true)
	return &Encoder{
		buf:     buf,
		Encoder: encoder,
	}
}

// Encode writes the JSON encoding of v to the stream.
func (enc *Encoder) Encode(v interface{}) ([]byte, error) {
	enc.buf.Reset()
	err := enc.Encoder.Encode(v)
	if err != nil {
		return nil, err
	}
	return enc.buf.Bytes(), nil
}

// NewDecoder returns a new decoder that reads from r.
func NewDecoder(r io.Reader) *Decoder {
	decoder := json.NewDecoder(r)
	decoder.DisallowUnknownFields()
	return &Decoder{Decoder: decoder}
}

// Decode reads the next JSON-encoded value from its
// input and stores it in the value pointed to by v.
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

// Marshal returns the JSON encoding of v.
func Marshal(v interface{}) ([]byte, error) {
	return NewEncoder(64).Encode(v)
}

// Unmarshal parses the JSON-encoded data and stores the result
// in the value pointed to by v. If v is nil or not a pointer,
// Unmarshal returns an InvalidUnmarshalError.
func Unmarshal(data []byte, v interface{}) error {
	return NewDecoder(bytes.NewReader(data)).Decode(v)
}
