package controller

import (
	"bytes"

	"github.com/pkg/errors"
	"github.com/vmihailenco/msgpack"

	"project/internal/bootstrap"
	"project/internal/config"
	"project/internal/convert"
	"project/internal/logger"
	"project/internal/messages"
	"project/internal/protocol"
)

// Trust_Node is used to trust Genesis Node
func (this *CTRL) Trust_Node(node *bootstrap.Node, listeners []*config.Listener) error {
	c := &client_cfg{Node: node}
	c.TLS_Config.InsecureSkipVerify = true
	client, err := new_client(this, c)
	if err != nil {
		return errors.Wrap(err, "connect node failed")
	}
	defer client.Close()
	// send trust node command
	reply, err := client.Send(protocol.CTRL_TRUST_NODE, nil)
	if err != nil {
		return errors.Wrap(err, "send trust node command failed")
	}
	req := &messages.Node_Online_Request{}
	err = msgpack.Unmarshal(reply, req)
	if err != nil {
		err = errors.Wrap(err, "invalid node online request")
		this.Print(logger.EXPLOIT, "trust_node", err)
		return err
	}
	err = req.Validate()
	if err != nil {
		err = errors.Wrap(err, "validate node online request failed")
		this.Print(logger.EXPLOIT, "trust_node", err)
		return err
	}
	// issue certificates
	// sign certificate with node guid
	buffer := bytes.Buffer{}
	buffer.WriteString(node.Mode)
	buffer.WriteString(node.Network)
	buffer.WriteString(node.Address)
	buffer.Write(req.GUID)
	cert_with_node_guid := this.global.Sign(buffer.Bytes())
	// sign certificate with controller guid
	buffer.Reset()
	buffer.WriteString(node.Mode)
	buffer.WriteString(node.Network)
	buffer.WriteString(node.Address)
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
	certificates := buffer.Bytes()
	// send response
	resp := &messages.Node_Online_Response{
		Listeners:    listeners, // TODO encrypt
		Certificates: certificates,
	}
	b, err := msgpack.Marshal(resp)
	if err != nil {
		panic(err)
	}
	reply, err = client.Send(protocol.CTRL_TRUST_NODE_DATA, b)
	if err != nil {
		return errors.Wrap(err, "send trust node data failed")
	}
	if !bytes.Equal(reply, messages.ONLINE_SUCCESS) {
		return errors.New("trust bootstrap faild")
	}
	// calculate aes key
	aes_key, err := this.global.Key_Exchange(req.Kex_Pub)
	if err != nil {
		panic(err)
	}
	// TODO broadcast

	// insert node
	return this.Insert_Node(&m_node{
		GUID:      req.GUID,
		Publickey: req.Publickey,
		AES_Key:   aes_key,
	})
}
