package trojan

import (
	"context"
	"io"
	"net"

	"github.com/MehranF123/sing/common"
	"github.com/MehranF123/sing/common/auth"
	"github.com/MehranF123/sing/common/buf"
	E "github.com/MehranF123/sing/common/exceptions"
	F "github.com/MehranF123/sing/common/format"
	M "github.com/MehranF123/sing/common/metadata"
	N "github.com/MehranF123/sing/common/network"
	"github.com/MehranF123/sing/common/rw"
)

type Handler interface {
	N.TCPConnectionHandler
	N.UDPConnectionHandler
}

type Service[K comparable] struct {
	handler Handler
	keys    map[[56]byte]K
	users   map[K][56]byte
}

func NewService[K comparable](handler Handler) Service[K] {
	return Service[K]{
		handler: handler,
		keys:    make(map[[56]byte]K),
		users:   make(map[K][56]byte),
	}
}

var ErrUserExists = E.New("user already exists")

func (s *Service[K]) AddUser(user K, password string) error {
	if _, loaded := s.users[user]; loaded {
		return ErrUserExists
	}
	key := Key(password)
	if oldUser, loaded := s.keys[key]; loaded {
		return E.New("password used by ", oldUser)
	}
	s.users[user] = key
	s.keys[key] = user
	return nil
}

func (s *Service[K]) RemoveUser(user K) bool {
	if key, loaded := s.users[user]; loaded {
		delete(s.users, user)
		delete(s.keys, key)
		return true
	}
	return false
}

func (s *Service[K]) ResetUsers() {
	s.keys = make(map[[56]byte]K)
	s.users = make(map[K][56]byte)
}

func (s *Service[K]) NewConnection(ctx context.Context, conn net.Conn, metadata M.Metadata) error {
	var key [KeyLength]byte
	_, err := io.ReadFull(conn, common.Dup(key[:]))
	if err != nil {
		return err
	}

	goto process

returnErr:
	err = &Error{
		Metadata: metadata,
		Conn:     conn,
		Inner:    err,
	}
	return err

process:

	if user, loaded := s.keys[key]; loaded {
		ctx = auth.ContextWithUser(ctx, user)
	} else {
		err = E.New("bad request")
		goto returnErr
	}

	err = rw.SkipN(conn, 2)
	if err != nil {
		err = E.Cause(err, "skip crlf")
		goto returnErr
	}

	command, err := rw.ReadByte(conn)
	if err != nil {
		err = E.Cause(err, "read command")
		goto returnErr
	}

	if command != CommandTCP && command != CommandUDP {
		err = E.New("unknown command ", command)
		goto returnErr
	}

	destination, err := M.SocksaddrSerializer.ReadAddrPort(conn)
	if err != nil {
		err = E.Cause(err, "read destination")
		goto returnErr
	}

	err = rw.SkipN(conn, 2)
	if err != nil {
		err = E.Cause(err, "skip crlf")
		goto returnErr
	}

	metadata.Protocol = "trojan"
	metadata.Destination = destination

	if command == CommandTCP {
		return s.handler.NewConnection(ctx, conn, metadata)
	} else {
		return s.handler.NewPacketConnection(ctx, &PacketConn{conn}, metadata)
	}
}

type PacketConn struct {
	net.Conn
}

func (c *PacketConn) ReadPacket(buffer *buf.Buffer) (M.Socksaddr, error) {
	return ReadPacket(c.Conn, buffer)
}

func (c *PacketConn) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	return WritePacket(c.Conn, buffer, destination)
}

type Error struct {
	Metadata M.Metadata
	Conn     net.Conn
	Inner    error
}

func (e *Error) Error() string {
	return F.ToString("process connection from ", e.Metadata.Source, ": ", e.Inner)
}

func (e *Error) Unwrap() error {
	return e.Inner
}

func (e *Error) Close() error {
	return e.Conn.Close()
}
