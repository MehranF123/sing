package bufio

import (
	"net"

	"github.com/MehranF123/sing/common/buf"
	M "github.com/MehranF123/sing/common/metadata"
	N "github.com/MehranF123/sing/common/network"
)

func Read(reader N.ExtendedReader, buffer *buf.Buffer) (n int, err error) {
	n, err = reader.Read(buffer.FreeBytes())
	buffer.Truncate(n)
	return
}

func ReadFrom(reader N.PacketReader, buffer *buf.Buffer) (n int, addr net.Addr, err error) {
	startLen := buffer.Len()
	addr, err = reader.ReadPacket(buffer)
	n = buffer.Len() - startLen
	return
}

func Write(writer N.ExtendedWriter, buffer *buf.Buffer) (n int, err error) {
	defer buffer.Release()
	dataLen := buffer.Len()
	err = writer.WriteBuffer(buffer)
	if err == nil {
		n = dataLen
	}
	return
}

func WriteTo(writer N.PacketWriter, buffer *buf.Buffer, addr net.Addr) (n int, err error) {
	defer buffer.Release()
	dataLen := buffer.Len()
	err = writer.WritePacket(buffer, M.SocksaddrFromNet(addr))
	if err == nil {
		n = dataLen
	}
	return
}
