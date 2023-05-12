package uot

import (
	M "github.com/MehranF123/sing/common/metadata"
)

const UOTMagicAddress = "sp.udp-over-tcp.arpa"

var AddrParser = M.NewSerializer(
	M.AddressFamilyByte(0x00, M.AddressFamilyIPv4),
	M.AddressFamilyByte(0x01, M.AddressFamilyIPv6),
	M.AddressFamilyByte(0x02, M.AddressFamilyFqdn),
)
