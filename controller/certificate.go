package controller

import (
	"bytes"
	"io"

	"project/internal/bootstrap"
	"project/internal/convert"
	"project/internal/protocol"
)

func (ctrl *CTRL) issueCertificate(node *bootstrap.Node, guid []byte) []byte {
	// sign certificate with node guid
	buffer := bytes.Buffer{}
	buffer.WriteString(node.Mode)
	buffer.WriteString(node.Network)
	buffer.WriteString(node.Address)
	buffer.Write(guid)
	certWithNodeGUID := ctrl.global.Sign(buffer.Bytes())
	// sign certificate with controller guid
	buffer.Truncate(len(node.Mode + node.Network + node.Address))
	buffer.Write(protocol.CtrlGUID)
	certWithCtrlGUID := ctrl.global.Sign(buffer.Bytes())
	// pack certificates
	// [2 byte uint16] size + signature with node guid
	// [2 byte uint16] size + signature with ctrl guid
	buffer.Reset()
	buffer.Write(convert.Uint16ToBytes(uint16(len(certWithNodeGUID))))
	buffer.Write(certWithNodeGUID)
	buffer.Write(convert.Uint16ToBytes(uint16(len(certWithCtrlGUID))))
	buffer.Write(certWithCtrlGUID)
	return buffer.Bytes()
}

func (ctrl *CTRL) verifyCertificate(cert []byte, node *bootstrap.Node, guid []byte) bool {
	// if guid = nil, skip verify
	if guid != nil {
		reader := bytes.NewReader(cert)
		// read certificate size
		certSize := make([]byte, 2)
		_, err := io.ReadFull(reader, certSize)
		if err != nil {
			return false
		}
		// read certificate with node guid
		certWithNodeGUID := make([]byte, convert.BytesToUint16(certSize))
		_, err = io.ReadFull(reader, certWithNodeGUID)
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
		if bytes.Equal(guid, protocol.CtrlGUID) {
			// read cert size
			_, err = io.ReadFull(reader, certSize)
			if err != nil {
				return false
			}
			// read cert
			certWithCtrlGUID := make([]byte, convert.BytesToUint16(certSize))
			_, err = io.ReadFull(reader, certWithCtrlGUID)
			if err != nil {
				return false
			}
			if !ctrl.global.Verify(buffer.Bytes(), certWithCtrlGUID) {
				return false
			}
		} else {
			if !ctrl.global.Verify(buffer.Bytes(), certWithNodeGUID) {
				return false
			}
		}
	}
	return true
}
