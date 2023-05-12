package exceptions

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"syscall"
	_ "unsafe"

	"github.com/MehranF123/sing/common"
	F "github.com/MehranF123/sing/common/format"
)

type Handler interface {
	NewError(ctx context.Context, err error)
}

type MultiError interface {
	UnwrapMulti() []error
}

func New(message ...any) error {
	return errors.New(F.ToString(message...))
}

func Cause(cause error, message ...any) error {
	return &causeError{F.ToString(message...), cause}
}

func Extend(cause error, message ...any) error {
	return &extendedError{F.ToString(message...), cause}
}

func Errors(errors ...error) error {
	errors = common.FilterNotNil(errors)
	errors = common.UniqBy(errors, error.Error)
	switch len(errors) {
	case 0:
		return nil
	case 1:
		return errors[0]
	}
	return &multiError{
		errors: errors,
	}
}

//go:linkname errCanceled net.errCanceled
var errCanceled error

func IsClosedOrCanceled(err error) bool {
	return IsMulti(err, io.EOF, net.ErrClosed, io.ErrClosedPipe, os.ErrClosed, syscall.EPIPE, syscall.ECONNRESET, context.Canceled, context.DeadlineExceeded, errCanceled)
}

func IsClosed(err error) bool {
	return IsMulti(err, io.EOF, net.ErrClosed, io.ErrClosedPipe, os.ErrClosed, syscall.EPIPE, syscall.ECONNRESET)
}

func IsCanceled(err error) bool {
	return IsMulti(err, context.Canceled, context.DeadlineExceeded, errCanceled)
}
