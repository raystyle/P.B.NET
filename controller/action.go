package controller

import (
	"bytes"
	"encoding/hex"
	"time"

	"github.com/pkg/errors"

	"project/internal/guid"
	"project/internal/module/info"
)

// hexByteSlice is used to marshal []byte to hex string.
type hexByteSlice []byte

// MarshalJSON is used to implement JSON Marshaler interface.
func (hs hexByteSlice) MarshalJSON() ([]byte, error) {
	const quotation = 34 // ASCII
	dst := make([]byte, 2*len(hs)+2)
	dst[0] = quotation
	hex.Encode(dst[1:], hs)
	dst[2*len(hs)+1] = quotation
	return bytes.ToUpper(dst), nil
}

// UnmarshalJSON is used to implement JSON Unmarshaler interface.
func (hs *hexByteSlice) UnmarshalJSON(data []byte) error {
	l := len(data)
	if l < 2 {
		return errors.New("invalid data about hex bytes slice")
	}
	*hs = make([]byte, (l-2)/2)
	_, err := hex.Decode(*hs, data[1:l-1])
	return err
}

// ------------------------------------------Node register-----------------------------------------

// see internal/messages/register.go

// NoticeNodeRegister is used to notice Node register to view.
type NoticeNodeRegister struct {
	ID           string       `json:"id"` // action id
	GUID         guid.GUID    `json:"guid"`
	PublicKey    hexByteSlice `json:"public_key"`
	KexPublicKey hexByteSlice `json:"kex_public_key"`
	ConnAddress  string       `json:"conn_address"`
	SystemInfo   *info.System `json:"system_info"`
	RequestTime  time.Time    `json:"request_time"`
}

// ReplyNodeRegister is used to reply Node register.
type ReplyNodeRegister struct {
	ID        string                 `json:"id"` // action id
	Result    uint8                  `json:"result"`
	Zone      string                 `json:"zone"`
	Listeners map[guid.GUID][]string `json:"listeners"` // Node listener tags
}

// -----------------------------------------Beacon register----------------------------------------

// NoticeBeaconRegister is used to notice Node register to view.
type NoticeBeaconRegister struct {
	ID           string       `json:"id"` // action id
	GUID         guid.GUID    `json:"guid"`
	PublicKey    hexByteSlice `json:"public_key"`
	KexPublicKey hexByteSlice `json:"kex_public_key"`
	ConnAddress  string       `json:"conn_address"`
	SystemInfo   *info.System `json:"system_info"`
	RequestTime  time.Time    `json:"request_time"`
}

// ReplyBeaconRegister is used to reply Node register.
type ReplyBeaconRegister struct {
	ID        string                 `json:"id"` // action id
	Result    uint8                  `json:"result"`
	Listeners map[guid.GUID][]string `json:"listeners"` // Beacon listener tags
}

// SelectedNodeListeners is used to control role connect Nodes.
// The fields above will overwrite the fields below except Listeners
// type SelectedNodeListeners struct {
// 	// the number of the selected Node listeners, default is 8.
// 	Number int `json:"number"`
//
// 	// if enable it, Controller will select random Nodes in random zones.
// 	AllRandomZone bool `json:"all_random"`
//
// 	// if enable it, Controller will select random Nodes in one selected random zone.
// 	OneRandomZone bool `json:"one_random_zone"`
//
// 	// if RandomNodes is not "", Controller will select random Nodes in selected zone.
// 	RandomNodes string `json:"random_nodes"`
//
// 	// select Nodes and Node listener tags manually.
// 	// key = hex(Node GUID), value = node listener tags
// 	Manually map[guid.GUID][]string `json:"manually"`
// }
