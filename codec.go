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
	_, err = e.writer.Write(bytes)
	return err
}

type RPC interface {
	Type() MethodType
	RPCData
}

type RPCData interface {
	Name() string
	Arg() interface{}
	SeqNo() seqNumber
	Err() error
	Res() interface{}
	Call() *call
	DataLength() int
	DecodeData(int, *codec.Decoder, *protocolHandler, *callContainer) error
	EncodeData([]interface{}) error
}

type BasicRPCData struct{}

func (BasicRPCData) Name() string {
	panic("not implemented for this type")
}

func (BasicRPCData) Arg() interface{} {
	panic("not implemented for this type")
}

func (BasicRPCData) SeqNo() seqNumber {
	panic("not implemented for this type")
}

func (BasicRPCData) Err() error {
	panic("not implemented for this type")
}

func (BasicRPCData) Res() interface{} {
	panic("not implemented for this type")
}

func (BasicRPCData) Call() *call {
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

func (r *RPCCallData) DecodeData(l int, d *codec.Decoder, p *protocolHandler, _ *callContainer) (err error) {
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
	return d.Decode(r.arg)
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

func (r *RPCNotifyData) DecodeData(l int, d *codec.Decoder, p *protocolHandler, _ *callContainer) (err error) {
	if l != r.DataLength() {
		return errors.New("wrong message length")
	}
	if err = d.Decode(&r.name); err != nil {
		return err
	}
	if r.arg, err = p.getArg(r.name); err != nil {
		return err
	}
	return d.Decode(r.arg)
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

func (r RPCNotifyData) SeqNo() seqNumber {
	return -1
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

func (c RPCCall) Encode(e *codec.Encoder) error {
	v := make([]interface{}, 1+c.DataLength())
	v[0] = c.typ
	if err := c.EncodeData(v[1:]); err != nil {
		return err
	}
	return e.Encode(v)
}

func (c *RPCCall) Decode(l int, d *codec.Decoder, p *protocolHandler, cc *callContainer) error {
	if err := d.Decode(&c.typ); err != nil {
		return err
	}

	switch c.typ {
	case MethodCall:
		c.RPCData = &RPCCallData{}
	case MethodNotify:
		c.RPCData = &RPCNotifyData{}
	case MethodResponse:
		c.RPCData = &RPCResponseData{}
	default:
		c.RPCData = nil
		return errors.New("invalid RPC type")
	}
	return c.DecodeData(l-1, d, p, cc)
}

type RPCResponseData struct {
	BasicRPCData
	c     *call
	seqNo seqNumber
	err   error
	res   interface{}
}

func (r RPCResponseData) DataLength() int {
	return 2
}

func (r RPCResponseData) EncodeData(v []interface{}) error {
	if len(v) != r.DataLength() {
		return errors.New("wrong message length")
	}
	// TODO finish
	return nil
}

func (r *RPCResponseData) DecodeData(l int, d *codec.Decoder, _ *protocolHandler, cc *callContainer) error {
	if l != 2 {
		return errors.New("wrong message length")
	}
	if err := d.Decode(&r.seqNo); err != nil {
		return err
	}

	// Attempt to retrieve the call
	r.c = cc.retrieveCall(r.seqNo)
	if r.c == nil {
		return errors.New("unable to retrieve call")
	}

	// Decode the error
	var responseErr interface{}
	if err := d.Decode(responseErr); err != nil {
		return err
	}
	if r.c.errorUnwrapper != nil {
		var dispatchErr error
		r.err, dispatchErr = r.c.errorUnwrapper.UnwrapError(responseErr)
		if dispatchErr != nil {
			return dispatchErr
		}
	} else {
		errAsString, ok := responseErr.(*string)
		if !ok {
			return errors.New("unable to convert error to string")
		}
		r.err = errors.New(*errAsString)
	}
	return d.Decode(r.res)
}

func (r RPCResponseData) SeqNo() seqNumber {
	return r.seqNo
}

func (r RPCResponseData) Err() error {
	return r.err
}

func (r RPCResponseData) Res() interface{} {
	return r.res
}

func (r RPCResponseData) Call() *call {
	return r.c
}
