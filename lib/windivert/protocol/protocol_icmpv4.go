package protocol

import (
	"encoding/binary"
)

const ICMPV4_HEAD_LEN = 8

func NewIcmpV4Protocol(parent Protocol) *icmpv4 {
	raw := parent.Content()
	head := raw[:ICMPV4_HEAD_LEN]

	return &icmpv4{
		ProtocolType:   ICMPV4,
		ParentProtocol: parent,
		raw:            raw,
		head:           head,
		content:        raw[ICMPV4_HEAD_LEN:],
		Type_:          head[0],
		Code:           head[1],
		Checksum:       binary.BigEndian.Uint16(head[2:4]),
		Body:           binary.BigEndian.Uint32(head[4:8]),
	}
}

type icmpv4 struct {
	ProtocolType   Type     `json:"protocolType"`
	ParentProtocol Protocol `json:"ParentProtocol,omitempty"`
	raw            []byte
	head           []byte
	content        []byte
	headLength     uint8
	Type_          uint8  `json:"Type"`
	Code           uint8  `json:"Code"`
	Checksum       uint16 `json:"Checksum"`
	Body           uint32 `json:"Body"`
}

func (icmpv4 *icmpv4) Type() Type {
	return icmpv4.ProtocolType
}

func (icmpv4 *icmpv4) Parent() Protocol {
	return icmpv4.ParentProtocol
}

func (icmpv4 *icmpv4) HeadLength() byte {
	return ICMPV4_HEAD_LEN
}

func (icmpv4 *icmpv4) Raw() []byte {
	return icmpv4.raw
}

func (icmpv4 *icmpv4) Head() []byte {
	return icmpv4.head
}

func (icmpv4 *icmpv4) Content() []byte {
	return icmpv4.content
}
