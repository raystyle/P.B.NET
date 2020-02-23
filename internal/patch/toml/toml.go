package toml

import (
	"fmt"
	"reflect"

	burn "github.com/BurntSushi/toml"
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
	metaData, err := burn.Decode(string(data), v)
	if err != nil {
		name := reflect.TypeOf(v).String()
		return fmt.Errorf("toml: %s in %s", err, name)
	}
	unDecoded := metaData.Undecoded()
	if len(unDecoded) != 0 {
		name := reflect.TypeOf(v).String()
		return fmt.Errorf("toml: %s not apply to %s", unDecoded[0], name)
	}
	srcTree, err := toml.LoadBytes(data)
	if err != nil {
		name := reflect.TypeOf(v).String()
		return fmt.Errorf("%s in %s", err, name)
	}
	return srcTree.Unmarshal(v)
}
