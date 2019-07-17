package messages

import (
	"github.com/pkg/errors"

	"project/internal/crypto/ed25519"
	"project/internal/guid"
	"project/internal/xnet"
)

const (
	NODE_ONLINE_REQUEST uint32 = 0x00000000 + iota
	NODE_ONLINE_RESPONSE
	BEACON_ONLINE_REQUEST
	BEACON_ONLINE_RESPONSE
)

type Online_Result uint8

const (
	ONLINE_ACCEPT Online_Result = iota
	ONLINE_REFUSED
	ONLINE_TIMEOUT
)

var (
	ONLINE_SUCCESS = []byte("ok")
)

type Host_Info struct {
	Internal_IP string
	Hostname    string
	Username    string
	PID         int
}

type Node_Online_Request struct {
	GUID      []byte
	Publickey []byte // verify message
	Kex_Pub   []byte // aes key exchange
	// about certificate
	Mode      xnet.Mode
	Network   string
	Host_Info Host_Info // online info session aes encrypt
	// more
	Request_Time int64
}

func (this *Node_Online_Request) Validate() error {
	if len(this.GUID) != guid.SIZE {
		return errors.New("invalid guid size")
	}
	if len(this.Publickey) != ed25519.PublicKey_Size {
		return errors.New("invalid publickey size")
	}
	if len(this.Kex_Pub) != 32 {
		return errors.New("invalid key exchange publickey size")
	}
	return xnet.Check_Mode_Network(this.Mode, this.Network)
}

type Node_Online_Response struct {
	// node info(for broadcast)
	GUID         []byte
	Publickey    []byte // verify message
	Kex_Pub      []byte // aes key exchange
	Mode         xnet.Mode
	Network      string
	Address      string
	Reply        Online_Result
	Request_Time int64
	Reply_Time   int64
	Certificate  []byte
}

func (this *Node_Online_Response) Validate() error {
	if len(this.GUID) != guid.SIZE {
		return errors.New("invalid guid size")
	}
	if this.Reply > ONLINE_REFUSED {
		return errors.New("invalid reply")
	}
	if this.Certificate == nil {
		return errors.New("invalid certificate")
	}
	return nil
}

type Beacon_Online_Request struct {
	GUID          []byte
	Session_AES   []byte // rsa encrypt
	Session_ECDSA []byte
	Request_Time  int64
	External_IP   string // node set
	Host_Info     []byte // online info session aes encrypt
}

func (this *Beacon_Online_Request) Validate() error {
	if len(this.GUID) != guid.SIZE {
		return errors.New("invalid guid size")
	}
	if this.Session_AES == nil {
		return errors.New("no session aes")
	}
	if this.Session_ECDSA == nil {
		return errors.New("no session ecdsa")
	}
	if this.External_IP == "" {
		return errors.New("no external ip")
	}
	if this.Host_Info == nil {
		return errors.New("no info")
	}
	return nil
}

type Beacon_Online_Response struct {
	GUID           []byte
	Session_AES    []byte // rsa encrypt
	Session_ECDSA  []byte
	Reply          Online_Result
	Request_Time   int64
	Confirmed_Time int64
}

func (this *Beacon_Online_Response) Validate() error {
	if len(this.GUID) != guid.SIZE {
		return errors.New("invalid guid size")
	}
	if this.Reply > ONLINE_REFUSED {
		return errors.New("invalid reply")
	}
	return nil
}
