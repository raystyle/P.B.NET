package rdpthief

import "github.com/Microsoft/go-winio"

func Listen() {
	winio.ListenPipe(`\\.\pipe\test`, nil)

}
