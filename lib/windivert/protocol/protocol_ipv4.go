package protocol

import (
	"encoding/binary"
	"net"
)

func NewIpV4Protocol(raw []byte) *ipv4 {
	headLength := (raw[0] & 0xF) << 2
	head := raw[:headLength]
	return &ipv4{
		ProtocolType: IPV4,
		raw:          raw,
		headLength:   headLength,
		head:         head,
		content:      raw[headLength:],
		Id:           binary.BigEndian.Uint16(head[4:6]),
		Checksum:     binary.BigEndian.Uint16(head[10:12]),
		SrcIp:        net.IPv4(head[12], head[13], head[14], head[15]),
		DstIP:        net.IPv4(head[16], head[17], head[18], head[18]),
	}
}

type ipv4 struct {
	ProtocolType Type `json:"protocolType"`
	raw          []byte
	head         []byte
	content      []byte
	headLength   uint8
	Id           uint16 `json:"id"`
	Checksum     uint16 `json:"checksum"`
	SrcIp        net.IP `json:"SrcIp"`
	DstIP        net.IP `json:"DstIP"`
}

func (ipv4 *ipv4) Type() Type {
	return ipv4.ProtocolType
}

func (ipv4 *ipv4) Parent() Protocol {
	return nil
}

func (ipv4 *ipv4) HeadLength() uint8 {
	return ipv4.headLength
}

func (ipv4 *ipv4) Raw() []byte {
	return ipv4.raw
}

func (ipv4 *ipv4) Head() []byte {
	return ipv4.head
}

func (ipv4 *ipv4) Content() []byte {
	return ipv4.content
}
