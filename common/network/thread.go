package network

import (
	"github.com/MehranF123/sing/common/buf"
	M "github.com/MehranF123/sing/common/metadata"
)

type ThreadUnsafeWriter interface {
	WriteIsThreadUnsafe()
}

type ThreadSafeReader interface {
	ReadBufferThreadSafe() (buffer *buf.Buffer, err error)
}

type ThreadSafePacketReader interface {
	ReadPacketThreadSafe() (buffer *buf.Buffer, addr M.Socksaddr, err error)
}
