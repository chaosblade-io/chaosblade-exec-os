package protocol

type Type uint8

const (
	IPV4 Type = 4
	IPV6 Type = 6

	ICMPV4 Type = 1
	ICMPV6 Type = 58

	TCP Type = 6
	UDP Type = 17
)

type Protocol interface {
	Parent() Protocol

	Type() Type

	HeadLength() uint8

	Raw() []byte

	Head() []byte

	Content() []byte
}

func ParseProtocol(data []byte) Protocol {
	v := data[0] >> 4

	var t Type
	var parentProtocol Protocol
	if v == 4 {
		t = Type(data[9])
		parentProtocol = NewIpV4Protocol(data)
	} else if v == 6 {
		t = Type(data[6])
		return nil
	}

	if t == ICMPV4 {
		return NewIcmpV4Protocol(parentProtocol)
	} else if t == TCP {
		return NewTcpProtocol(parentProtocol)
	}

	return nil
}
