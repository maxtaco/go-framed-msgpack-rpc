package rpc2

import (
	"bufio"
	"bytes"
	"errors"
	"github.com/ugorji/go/codec"
	"io/ioutil"
	"net"
)

type WrapErrorFunc func(error) interface{}
type UnwrapErrorFunc func(nxt DecodeNext) (error, error)

type Transporter interface {
	RawWrite([]byte) error
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
	br          *bufio.Reader
	buf         *bytes.Buffer
	enc         *codec.Encoder
	dec         *codec.Decoder
}

func NewTransport(c net.Conn, w WrapErrorFunc, u UnwrapErrorFunc) *Transport {
	var mh codec.MsgpackHandle
	buf := new(bytes.Buffer)
	br := bufio.NewReader(c)
	return &Transport{
		con:         c,
		wrapError:   w,
		unwrapError: u,
		br:          br,
		buf:         buf,
		enc:         codec.NewEncoder(buf, &mh),
		dec:         codec.NewDecoder(br, &mh),
	}
}

func (t *Transport) encodeToBytes(i interface{}) (v []byte, err error) {
	if err = t.enc.Encode(i); err != nil {
		return
	}
	v, _ = ioutil.ReadAll(t.buf)
	return
}

func (t *Transport) Encode(i interface{}) (err error) {
	var v1, v2 []byte
	if v2, err = t.encodeToBytes(i); err != nil {
		return
	}
	l := len(v2)
	if v1, err = t.encodeToBytes(l); err != nil {
		return
	}
	if err = t.RawWrite(v1); err != nil {
		return
	}
	return t.RawWrite(v2)
}

func (t *Transport) WrapError(e error) interface{} {
	if t.wrapError != nil {
		return t.wrapError(e)
	} else {
		return e.Error()
	}
}

func (t *Transport) ReadByte() (byte, error) {
	return t.br.ReadByte()
}

func (t *Transport) Decode(i interface{}) error {
	return t.dec.Decode(i)
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

func (t *Transport) RawWrite(b []byte) error {
	_, err := t.con.Write(b)
	return err
}
