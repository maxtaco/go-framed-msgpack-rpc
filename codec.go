package rpc

import (
	"errors"
	"io"

	"github.com/ugorji/go/codec"
)

type decoder interface {
	Decode(interface{}) error
}

type byteReadingDecoder interface {
	decoder
	io.ByteReader
}

type encoder interface {
	Encode(interface{}) error
}

func newCodecMsgpackHandle() codec.Handle {
	return &codec.MsgpackHandle{
		WriteExt: true,
	}
}

type framedMsgpackEncoder struct {
	enc    *codec.Encoder
	writer io.Writer
}

func newFramedMsgpackEncoder(writer io.Writer) *framedMsgpackEncoder {
	return &framedMsgpackEncoder{
		enc:    codec.NewEncoderBytes(&[]byte{}, newCodecMsgpackHandle()),
		writer: writer,
	}
}

func (e *framedMsgpackEncoder) encodeToBytes(i interface{}) (v []byte, err error) {
	e.enc.ResetBytes(&v)
	err = e.enc.Encode(i)
	return v, err
}

func (e *framedMsgpackEncoder) encodeFrame(i interface{}) ([]byte, error) {
	content, err := e.encodeToBytes(i)
	if err != nil {
		return nil, err
	}
	length, err := e.encodeToBytes(len(content))
	if err != nil {
		return nil, err
	}
	return append(length, content...), nil
}

func (e *framedMsgpackEncoder) Encode(i interface{}) error {
	bytes, err := e.encodeFrame(i)
	if err != nil {
		return err
	}
	n, err := e.writer.Write(bytes)
	return err
}

type RPC interface {
	Type() MethodType
	RPCData
}

type RPCData interface {
	Name() string
	Arg() interface{}
	Res() interface{}
	SeqNo() seqNumber
	DataLength() int
	DecodeData(int, *codec.Decoder, *protocolHandler) error
	EncodeData([]interface{}) error
}

type BasicRPCData struct {
	//TODO fix this
}

func (BasicRPCData) Name() string {
	panic("not implemented for this type")
}

func (BasicRPCData) Arg() interface{} {
	panic("not implemented for this type")
}

func (BasicRPCData) Res() interface{} {
	panic("not implemented for this type")
}

func (BasicRPCData) SeqNo() seqNumber {
	panic("not implemented for this type")
}

type RPCCallData struct {
	BasicRPCData
	seqno seqNumber
	name  string
	arg   interface{}
}

func (RPCCallData) DataLength() int {
	return 3
}

func (r RPCCallData) DecodeData(l int, d *codec.Decoder, p *protocolHandler) (err error) {
	if l != r.DataLength() {
		return errors.New("wrong message length")
	}
	if err = d.Decode(&r.seqno); err != nil {
		return err
	}
	if err = d.Decode(&r.name); err != nil {
		return err
	}
	if r.arg, err = p.getArg(r.name); err != nil {
		return err
	}
	if err = d.Decode(r.arg); err != nil {
		return err
	}
	return nil
}

func (r RPCCallData) EncodeData(v []interface{}) error {
	if len(v) != r.DataLength() {
		return errors.New("wrong message length")
	}
	v[0] = r.seqno
	v[1] = r.name
	v[2] = r.arg
	return nil
}

func (r RPCCallData) SeqNo() seqNumber {
	return r.seqno
}

func (r RPCCallData) Name() string {
	return r.name
}

func (r RPCCallData) Arg() interface{} {
	return r.arg
}

type RPCNotifyData struct {
	BasicRPCData
	name string
	arg  interface{}
}

func (r RPCNotifyData) DecodeData(l int, d *codec.Decoder, p *protocolHandler) (err error) {
	if l != r.DataLength() {
		return errors.New("wrong message length")
	}
	if err = d.Decode(&r.name); err != nil {
		return err
	}
	if r.arg, err = p.getArg(r.name); err != nil {
		return err
	}
	if err = d.Decode(r.arg); err != nil {
		return err
	}
	return nil
}

func (r RPCNotifyData) EncodeData(v []interface{}) error {
	if len(v) != r.DataLength() {
		return errors.New("wrong message length")
	}
	v[0] = r.name
	v[1] = r.arg
	return nil
}

func (RPCNotifyData) DataLength() int {
	return 2
}

func (r RPCNotifyData) Name() string {
	return r.name
}

func (r RPCNotifyData) Arg() interface{} {
	return r.arg
}

type RPCCall struct {
	typ MethodType

	RPCData
}

func (c RPCCall) Type() MethodType {
	return c.typ
}

func (c RPCCall) Encode(e *codec.Encoder) {
	v := make([]interface{}, 1+c.DataLength())
	v[0] = c.typ
	c.EncodeData(v[1:])
	e.Encode(v)
}

func (c *RPCCall) Decode(l int, d *codec.Decoder, p *protocolHandler) error {
	if err := d.Decode(&c.typ); err != nil {
		return err
	}

	switch c.typ {
	case MethodCall:
		c.RPCData = RPCCallData{}
	case MethodNotify:
		c.RPCData = RPCNotifyData{}
	default:
		c.RPCData = nil
		return errors.New("invalid RPC type")
	}
	return c.DecodeData(l-1, d, p)
}
