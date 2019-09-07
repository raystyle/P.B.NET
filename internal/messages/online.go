package messages

import (
	"github.com/pkg/errors"

	"project/internal/config"
	"project/internal/crypto/ed25519"
	"project/internal/guid"
)

const (
	MessageNodeOnlineRequest uint32 = 0x00000000 + iota
	MessageNodeOnlineResponse
	MessageBeaconOnlineRequest
	MessageBeaconOnlineResponse
)

const (
	OnlineAccept uint8 = iota
	OnlineRefused
	OnlineTimeout
)

var (
	OnlineSucceed = []byte("ok")
)

type HostInfo struct {
	InternalIP string
	Hostname   string
	Username   string
	PID        int
}

type NodeOnlineRequest struct {
	GUID         []byte
	PublicKey    []byte   // verify message
	KexPublicKey []byte   // aes key exchange
	HostInfo     HostInfo // online info session aes encrypt
	RequestTime  int64
}

func (n *NodeOnlineRequest) Validate() error {
	if len(n.GUID) != guid.SIZE {
		return errors.New("invalid guid size")
	}
	if len(n.PublicKey) != ed25519.PublicKeySize {
		return errors.New("invalid public key size")
	}
	if len(n.KexPublicKey) != 32 {
		return errors.New("invalid key exchange public key size")
	}
	return nil
}

// node info(for broadcast)
type NodeOnlineResponse struct {
	GUID         []byte
	PublicKey    []byte // verify message
	KexPublicKey []byte // aes key exchange
	Listeners    []*config.Listener
	Result       uint8 // accept refused timeout
	RequestTime  int64
	ReplyTime    int64
	Certificates []byte
}

func (n *NodeOnlineResponse) Validate() error {
	if len(n.GUID) != guid.SIZE {
		return errors.New("invalid guid size")
	}
	if n.Result > OnlineRefused {
		return errors.New("invalid reply")
	}
	if n.Certificates == nil {
		return errors.New("no certificate")
	}
	return nil
}

type BeaconOnlineRequest struct {
	GUID          []byte
	Session_AES   []byte // rsa encrypt
	Session_ECDSA []byte
	Request_Time  int64
	External_IP   string // node set
	Host_Info     []byte // online info session aes encrypt
}

func (this *BeaconOnlineRequest) Validate() error {
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
	Reply          uint8
	Request_Time   int64
	Confirmed_Time int64
}

func (this *Beacon_Online_Response) Validate() error {
	if len(this.GUID) != guid.SIZE {
		return errors.New("invalid guid size")
	}
	if this.Reply > OnlineRefused {
		return errors.New("invalid reply")
	}
	return nil
}
