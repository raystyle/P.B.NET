package dns

import (
	"bytes"
	"encoding/binary"
	"strings"

	"project/internal/random"
)

type dns_header struct {
	id             uint16
	flag           uint16
	question_count uint16
	answerrrs      uint16
	authorityrrs   uint16
	additionalrrs  uint16
}

func (header *dns_header) set_flag(QR uint16, OperationCode uint16,
	AuthoritativeAnswer uint16, Truncation uint16, RecursionDesired uint16,
	RecursionAvailable uint16, ResponseCode uint16) {
	header.flag = QR<<15 + OperationCode<<11 + AuthoritativeAnswer<<10 +
		Truncation<<9 + RecursionDesired<<8 + RecursionAvailable<<7 +
		ResponseCode
}

func pack_question(question_type uint16, domain_name string) []byte {
	// add dns header
	dns_header := &dns_header{
		id:             uint16(random.Int(65535)),
		question_count: 1,
		answerrrs:      0,
		authorityrrs:   0,
		additionalrrs:  0,
	}
	dns_header.set_flag(0, 0,
		0, 0, 1,
		0, 0)
	// add dns question header
	dns_question := &struct {
		question_type  uint16
		question_class uint16
	}{
		question_type,
		1,
	}
	buffer := &bytes.Buffer{}
	_ = binary.Write(buffer, binary.BigEndian, dns_header)
	_ = binary.Write(buffer, binary.BigEndian, pack_domain_name(domain_name))
	_ = binary.Write(buffer, binary.BigEndian, dns_question)
	return buffer.Bytes()
}

func pack_domain_name(domain string) []byte {
	buffer := &bytes.Buffer{}
	segments := strings.Split(domain, ".")
	for _, seg := range segments {
		_ = binary.Write(buffer, binary.BigEndian, byte(len(seg)))
		_ = binary.Write(buffer, binary.BigEndian, []byte(seg))
	}
	_ = binary.Write(buffer, binary.BigEndian, byte(0x00))
	return buffer.Bytes()
}
