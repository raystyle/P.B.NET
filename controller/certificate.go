package controller

import (
	"bytes"
	"io"

	"project/internal/bootstrap"
	"project/internal/convert"
	"project/internal/protocol"
)

func (this *CTRL) issue_certificate(guid []byte, node *bootstrap.Node) []byte {
	// sign certificate with node guid
	buffer := bytes.Buffer{}
	buffer.WriteString(node.Mode)
	buffer.WriteString(node.Network)
	buffer.WriteString(node.Address)
	buffer.Write(guid)
	cert_with_node_guid := this.global.Sign(buffer.Bytes())
	// sign certificate with controller guid
	buffer.Truncate(len(node.Mode + node.Network + node.Address))
	buffer.Write(protocol.CTRL_GUID)
	cert_with_ctrl_guid := this.global.Sign(buffer.Bytes())
	// pack certificates
	// [2 byte uint16] size + signature with node guid
	// [2 byte uint16] size + signature with ctrl guid
	buffer.Reset()
	buffer.Write(convert.Uint16_Bytes(uint16(len(cert_with_node_guid))))
	buffer.Write(cert_with_node_guid)
	buffer.Write(convert.Uint16_Bytes(uint16(len(cert_with_ctrl_guid))))
	buffer.Write(cert_with_ctrl_guid)
	return buffer.Bytes()
}

func (this *CTRL) verify_certificate(cert, guid []byte, node *bootstrap.Node) bool {
	// if guid = nil, skip verify
	if guid != nil {
		reader := bytes.NewReader(cert)
		// read certificate size
		cert_size := make([]byte, 2)
		_, err := io.ReadFull(reader, cert_size)
		if err != nil {
			return false
		}
		// read certificate with node guid
		cert_with_node_guid := make([]byte, convert.Bytes_Uint16(cert_size))
		_, err = io.ReadFull(reader, cert_with_node_guid)
		if err != nil {
			return false
		}
		// verify certificate
		buffer := bytes.Buffer{}
		buffer.WriteString(node.Mode)
		buffer.WriteString(node.Network)
		buffer.WriteString(node.Address)
		buffer.Write(guid)
		// switch certificate
		if bytes.Equal(guid, protocol.CTRL_GUID) {
			// read cert size
			_, err = io.ReadFull(reader, cert_size)
			if err != nil {
				return false
			}
			// read cert
			cert_with_ctrl_guid := make([]byte, convert.Bytes_Uint16(cert_size))
			_, err = io.ReadFull(reader, cert_with_ctrl_guid)
			if err != nil {
				return false
			}
			if !this.global.Verify(buffer.Bytes(), cert_with_ctrl_guid) {
				return false
			}
		} else {
			if !this.global.Verify(buffer.Bytes(), cert_with_node_guid) {
				return false
			}
		}
	}
	return true
}
