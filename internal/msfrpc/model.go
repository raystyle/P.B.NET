package msfrpc

// MSFError is an error about Metasploit RPC.
type MSFError struct {
	Err            bool     `msgpack:"error"`
	ErrorClass     string   `msgpack:"error_class"`
	ErrorString    string   `msgpack:"error_string"`
	ErrorBacktrace []string `msgpack:"error_backtrace"`
	ErrorMessage   string   `msgpack:"error_message"`
	ErrorCode      int      `msgpack:"error_code"`
}

func (err *MSFError) Error() string {
	return err.ErrorMessage
}

// for msgpack marshal as array.
type asArray interface {
	asArray()
}

const success = "success"

// ------------------------------------------about methods-----------------------------------------
const (
	MethodAuthLogin  = "auth.login"
	MethodAuthLogout = "auth.logout"
)

// ------------------------------------------about models------------------------------------------

// AuthLoginRequest is used to login and get token.
type AuthLoginRequest struct {
	Method   string
	Username string
	Password string
}

func (alr *AuthLoginRequest) asArray() {}

// AuthLoginResult is the result about login.
type AuthLoginResult struct {
	Result string `msgpack:"result"`
	Token  string `msgpack:"token"`
	MSFError
}

// AuthLogoutRequest is used to delete token.
type AuthLogoutRequest struct {
	Method      string
	Token       string
	LogoutToken string // will be deleted
}

func (alr *AuthLogoutRequest) asArray() {}

// AuthLogoutResult is the result about logout.
type AuthLogoutResult struct {
	Result string `msgpack:"result"`
	MSFError
}
