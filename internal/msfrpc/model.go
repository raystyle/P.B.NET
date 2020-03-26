package msfrpc

type msfError struct {
	Err          bool   `msgpack:"error,omitempty"`
	ErrorClass   string `msgpack:"error_class,omitempty"`
	ErrorMessage string `msgpack:"error_message,omitempty"`
}

func (err *msfError) Error() string {
	return err.ErrorClass + " " + err.ErrorMessage
}
