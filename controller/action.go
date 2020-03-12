package controller

import (
	"encoding/hex"
	"time"

	"github.com/pkg/errors"

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
	return dst, nil
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
	GUID         hexByteSlice `json:"guid"`
	PublicKey    hexByteSlice `json:"public_key"`
	KexPublicKey hexByteSlice `json:"kex_public_key"`
	ConnAddress  string       `json:"conn_address"`
	SystemInfo   *info.System `json:"system_info"`
	RequestTime  time.Time    `json:"request_time"`
}

// ReplyNodeRegister is used to reply Node register.
type ReplyNodeRegister struct {
	ID     string `json:"id"` // action id
	Result uint8  `json:"result"`

	// key = hex(Node GUID), value = node listener tags
	NodeListeners map[string][]string `json:"node_listeners"`
}

// -----------------------------------------Beacon register----------------------------------------

// NoticeBeaconRegister is used to notice Node register to view.
type NoticeBeaconRegister struct {
	ID           string       `json:"id"` // action id
	GUID         hexByteSlice `json:"guid"`
	PublicKey    hexByteSlice `json:"public_key"`
	KexPublicKey hexByteSlice `json:"kex_public_key"`
	ConnAddress  string       `json:"conn_address"`
	SystemInfo   *info.System `json:"system_info"`
	RequestTime  time.Time    `json:"request_time"`
}

// ReplyBeaconRegister is used to reply Node register.
type ReplyBeaconRegister struct {
	ID     string `json:"id"` // action id
	Result uint8  `json:"result"`

	// key = hex(Node GUID), value = node listener tags
	NodeListeners map[string][]string `json:"node_listeners"`
}
