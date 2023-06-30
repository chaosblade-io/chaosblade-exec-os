package protocol

import (
	"encoding/binary"
)

func NewTcpProtocol(parent Protocol) *tcp {
	raw := parent.Content()
	headLength := (raw[12] >> 4) * 4
	head := raw[:headLength]

	return &tcp{
		ProtocolType: TCP,
		headLength:   headLength,
		raw:          raw,
		head:         head,
		content:      raw[headLength:],
		SrcPort:      binary.BigEndian.Uint16(head[0:2]),
		DstPort:      binary.BigEndian.Uint16(head[2:4]),
		Seq:          binary.BigEndian.Uint16(head[4:8]),
		Ack:          binary.BigEndian.Uint16(head[8:12]),
		Reserved:     (head[12] >> 1) & 0x7,
	}
}

type tcp struct {
	ProtocolType Type `json:"protocolType"`
	parent       Protocol
	raw          []byte
	head         []byte
	content      []byte
	headLength   uint8
	SrcPort      uint16 `json:"SrcPort"`
	DstPort      uint16 `json:"DstPort"`
	Seq          uint16 `json:"Seq"`
	Ack          uint16 `json:"Ack"`
	Reserved     byte   `json:"Reserved"`
}

func (tcp *tcp) Type() Type {
	return ICMPV4
}

func (tcp *tcp) Parent() Protocol {
	return tcp.parent
}

func (tcp *tcp) HeadLength() uint8 {
	return tcp.headLength
}

func (tcp *tcp) Raw() []byte {
	return tcp.raw
}

func (tcp *tcp) Head() []byte {
	return tcp.head
}

func (tcp *tcp) Content() []byte {
	return tcp.content
}
