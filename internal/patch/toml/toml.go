package toml

import (
	"github.com/pelletier/go-toml"
)

// Marshal returns the TOML encoding of v.
func Marshal(v interface{}) ([]byte, error) {
	return toml.Marshal(v)
}

// Unmarshal parses the TOML-encoded data and stores the result in the value.
// if field in source toml data doesn't exist in destination structure,
// it will return a error that include the tag.
func Unmarshal(data []byte, v interface{}) error {
	srcTree, err := toml.LoadBytes(data)
	if err != nil {
		return err
	}
	// srcKeys := srcTree.Keys()
	// fmt.Println(srcKeys)
	// check tag is exist
	// dstData, err := toml.Marshal(v)
	// if err != nil {
	// 	return err
	// }
	// dstTree, _ := toml.LoadBytes(dstData)
	// fmt.Println(dstTree.Keys())
	// for i := 0; i < len(srcKeys); i++ {
	// 	srcKey := srcKeys[i]
	// 	if !dstTree.Has(srcKey) {
	// 		sName := xreflect.GetStructureName(v)
	// 		return fmt.Errorf("unknown field: %s in %s", srcKey, sName)
	// 	}
	// }
	return srcTree.Unmarshal(v)
}
