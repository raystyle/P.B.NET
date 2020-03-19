package messages

import (
	"project/internal/guid"
)

func testGenerateGUID() *guid.GUID {
	g := guid.GUID{}
	g[0] = 1
	return &g
}
