package windivert

type WinDivertAddress struct {
	Timestamp         int64
	IfIdx             uint32
	SubIfIdx          uint32
	Direction         uint8
	Loopback          uint8
	Impostor          uint8
	PseudoIPChecksum  uint8
	PseudoTCPChecksum uint8
	PseudoUDPChecksum uint8
}
