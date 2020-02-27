package toml

import (
	"bytes"
	"fmt"
	"reflect"

	"github.com/pelletier/go-toml"
)

// Marshal returns the TOML encoding of v.
func Marshal(v interface{}) ([]byte, error) {
	return toml.Marshal(v)
}

// Unmarshal parses the TOML-encoded data and stores the result in the value.
// if field in source toml data doesn't exist in destination structure,
// it will return a error that include the field.
func Unmarshal(data []byte, v interface{}) error {
	decoder := toml.NewDecoder(bytes.NewReader(data)).Strict(true)
	err := decoder.Decode(v)
	if err != nil {
		name := reflect.TypeOf(v).String()
		return fmt.Errorf("toml: %s in %s", err, name)
	}
	return nil
}
