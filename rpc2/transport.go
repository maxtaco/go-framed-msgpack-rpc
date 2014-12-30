package rpc2

import (
	"errors"
	"net"
)

type WrapErrorFunc func(error) interface{}
type UnwrapErrorFunc func(nxt DecodeNext) (error, error)

type Transporter interface {
	RawWrite([]byte) error
	PacketizerError(error)
	ReadByte() (byte, error)
	Decode(interface{}) error
	WrapError(error) interface{}
	Encode(interface{}) error
	UnwrapError(DecodeNext) (error, error)
}

type Transport struct {
	con         net.Conn
	wrapError   WrapErrorFunc
	unwrapError UnwrapErrorFunc
}

func NewTransport(c net.Conn, w WrapErrorFunc, u UnwrapErrorFunc) *Transport {
	return &Transport{
		con:         c,
		wrapError:   w,
		unwrapError: u,
	}
}

func (t *Transport) WrapError(e error) interface{} {
	if t.wrapError != nil {
		return t.wrapError(e)
	} else {
		return e.Error()
	}
}

func (t *Transport) UnwrapError(dnf DecodeNext) (app error, dispatch error) {
	var s string
	if t.unwrapError != nil {
		app, dispatch = t.unwrapError(dnf)
	} else if dispatch = dnf(&s); dispatch == nil {
		app = errors.New(s)
	}
	return
}
