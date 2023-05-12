package trojan

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"io"
	"net"

	"github.com/MehranF123/sing/common"
	"github.com/MehranF123/sing/common/buf"
	"github.com/MehranF123/sing/common/bufio"
	E "github.com/MehranF123/sing/common/exceptions"
	M "github.com/MehranF123/sing/common/metadata"
	"github.com/MehranF123/sing/common/rw"
)

const (
	KeyLength  = 56
	CommandTCP = 1
	CommandUDP = 3
)

var CRLF = []byte{'\r', '\n'}

type ClientConn struct {
	net.Conn
	key           [KeyLength]byte
	destination   M.Socksaddr
	headerWritten bool
}

func NewClientConn(conn net.Conn, key [KeyLength]byte, destination M.Socksaddr) *ClientConn {
	return &ClientConn{
		Conn:        conn,
		key:         key,
		destination: destination,
	}
}

func (c *ClientConn) Write(p []byte) (n int, err error) {
	if c.headerWritten {
		return c.Conn.Write(p)
	}
	err = ClientHandshake(c.Conn, c.key, c.destination, p)
	if err != nil {
		return
	}
	n = len(p)
	c.headerWritten = true
	return
}

func (c *ClientConn) WriteBuffer(buffer *buf.Buffer) error {
	defer buffer.Release()
	if c.headerWritten {
		return common.Error(c.Conn.Write(buffer.Bytes()))
	}
	err := ClientHandshakeBuffer(c.Conn, c.key, c.destination, buffer)
	if err != nil {
		return err
	}
	c.headerWritten = true
	return nil
}

func (c *ClientConn) ReadFrom(r io.Reader) (n int64, err error) {
	if !c.headerWritten {
		return bufio.ReadFrom0(c, r)
	}
	return bufio.Copy(c.Conn, r)
}

func (c *ClientConn) WriteTo(w io.Writer) (n int64, err error) {
	return bufio.Copy(w, c.Conn)
}

type ClientPacketConn struct {
	net.Conn
	key           [KeyLength]byte
	headerWritten bool
}

func NewClientPacketConn(conn net.Conn, key [KeyLength]byte) *ClientPacketConn {
	return &ClientPacketConn{
		Conn: conn,
		key:  key,
	}
}

func (c *ClientPacketConn) ReadPacket(buffer *buf.Buffer) (M.Socksaddr, error) {
	return ReadPacket(c.Conn, buffer)
}

func (c *ClientPacketConn) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	defer buffer.Release()
	if !c.headerWritten {
		err := ClientHandshakePacket(c.Conn, c.key, destination, buffer)
		c.headerWritten = true
		return err
	}
	return WritePacket(c.Conn, buffer, destination)
}

func (c *ClientPacketConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	buffer := buf.With(p)
	destination, err := c.ReadPacket(buffer)
	if err != nil {
		return
	}
	n = buffer.Len()
	addr = destination.UDPAddr()
	return
}

func (c *ClientPacketConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	err = c.WritePacket(buf.With(p), M.SocksaddrFromNet(addr))
	if err == nil {
		n = len(p)
	}
	return
}

func Key(password string) [KeyLength]byte {
	var key [KeyLength]byte
	hash := sha256.New224()
	common.Must1(hash.Write([]byte(password)))
	hex.Encode(key[:], hash.Sum(nil))
	return key
}

func ClientHandshakeRaw(conn net.Conn, key [KeyLength]byte, command byte, destination M.Socksaddr, payload []byte) error {
	_, err := conn.Write(key[:])
	if err != nil {
		return err
	}
	_, err = conn.Write(CRLF)
	if err != nil {
		return err
	}
	_, err = conn.Write([]byte{command})
	if err != nil {
		return err
	}
	err = M.SocksaddrSerializer.WriteAddrPort(conn, destination)
	if err != nil {
		return err
	}
	_, err = conn.Write(CRLF)
	if err != nil {
		return err
	}
	if len(payload) > 0 {
		_, err = conn.Write(payload)
		if err != nil {
			return err
		}
	}
	return nil
}

func ClientHandshake(conn net.Conn, key [KeyLength]byte, destination M.Socksaddr, payload []byte) error {
	headerLen := KeyLength + M.SocksaddrSerializer.AddrPortLen(destination) + 5
	var header *buf.Buffer
	defer header.Release()
	var writeHeader bool
	if len(payload) > 0 && headerLen+len(payload) < 65535 {
		buffer := buf.StackNewSize(headerLen + len(payload))
		defer common.KeepAlive(buffer)
		header = common.Dup(buffer)
	} else {
		buffer := buf.StackNewSize(headerLen)
		defer common.KeepAlive(buffer)
		header = common.Dup(buffer)
		writeHeader = true
	}
	common.Must1(header.Write(key[:]))
	common.Must1(header.Write(CRLF))
	common.Must(header.WriteByte(CommandTCP))
	common.Must(M.SocksaddrSerializer.WriteAddrPort(header, destination))
	common.Must1(header.Write(CRLF))
	if !writeHeader {
		common.Must1(header.Write(payload))
	}

	_, err := conn.Write(header.Bytes())
	if err != nil {
		return E.Cause(err, "write request")
	}

	if writeHeader {
		_, err = conn.Write(payload)
		if err != nil {
			return E.Cause(err, "write payload")
		}
	}
	return nil
}

func ClientHandshakeBuffer(conn net.Conn, key [KeyLength]byte, destination M.Socksaddr, payload *buf.Buffer) error {
	header := buf.With(payload.ExtendHeader(KeyLength + M.SocksaddrSerializer.AddrPortLen(destination) + 5))
	common.Must1(header.Write(key[:]))
	common.Must1(header.Write(CRLF))
	common.Must(header.WriteByte(CommandTCP))
	common.Must(M.SocksaddrSerializer.WriteAddrPort(header, destination))
	common.Must1(header.Write(CRLF))

	_, err := conn.Write(payload.Bytes())
	if err != nil {
		return E.Cause(err, "write request")
	}
	return nil
}

func ClientHandshakePacket(conn net.Conn, key [KeyLength]byte, destination M.Socksaddr, payload *buf.Buffer) error {
	headerLen := KeyLength + 2*M.SocksaddrSerializer.AddrPortLen(destination) + 9
	payloadLen := payload.Len()
	var header *buf.Buffer
	defer header.Release()
	var writeHeader bool
	if payload.Start() >= headerLen {
		header = buf.With(payload.ExtendHeader(headerLen))
	} else {
		buffer := buf.StackNewSize(headerLen)
		defer common.KeepAlive(buffer)
		header = common.Dup(buffer)
		writeHeader = true
	}
	common.Must1(header.Write(key[:]))
	common.Must1(header.Write(CRLF))
	common.Must(header.WriteByte(CommandUDP))
	common.Must(M.SocksaddrSerializer.WriteAddrPort(header, destination))
	common.Must1(header.Write(CRLF))
	common.Must(M.SocksaddrSerializer.WriteAddrPort(header, destination))
	common.Must(binary.Write(header, binary.BigEndian, uint16(payloadLen)))
	common.Must1(header.Write(CRLF))

	if writeHeader {
		_, err := conn.Write(header.Bytes())
		if err != nil {
			return E.Cause(err, "write request")
		}
	}

	_, err := conn.Write(payload.Bytes())
	if err != nil {
		return E.Cause(err, "write payload")
	}
	return nil
}

func ReadPacket(conn net.Conn, buffer *buf.Buffer) (M.Socksaddr, error) {
	destination, err := M.SocksaddrSerializer.ReadAddrPort(conn)
	if err != nil {
		return M.Socksaddr{}, E.Cause(err, "read destination")
	}

	var length uint16
	err = binary.Read(conn, binary.BigEndian, &length)
	if err != nil {
		return M.Socksaddr{}, E.Cause(err, "read chunk length")
	}

	if buffer.FreeLen() < int(length) {
		return M.Socksaddr{}, io.ErrShortBuffer
	}

	err = rw.SkipN(conn, 2)
	if err != nil {
		return M.Socksaddr{}, E.Cause(err, "skip crlf")
	}

	_, err = buffer.ReadFullFrom(conn, int(length))
	return destination, err
}

func WritePacket(conn net.Conn, buffer *buf.Buffer, destination M.Socksaddr) error {
	defer buffer.Release()
	bufferLen := buffer.Len()
	header := buf.With(buffer.ExtendHeader(M.SocksaddrSerializer.AddrPortLen(destination) + 4))
	common.Must(M.SocksaddrSerializer.WriteAddrPort(header, destination))
	common.Must(binary.Write(header, binary.BigEndian, uint16(bufferLen)))
	common.Must1(header.Write(CRLF))
	_, err := conn.Write(buffer.Bytes())
	if err != nil {
		return E.Cause(err, "write packet")
	}
	return nil
}
