package rpc

import (
	"errors"
)

type RPCMessage interface {
	Type() MethodType
	Encode(e encoder) error
	Decode(l int, d decoder, p *protocolHandler, cc *callContainer) error
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
	DecodeData(int, decoder, *protocolHandler, *callContainer) error
	EncodeData([]interface{}) error
}

type BasicRPCData struct{}

func (BasicRPCData) Name() string {
	panic("Name() not implemented for this type")
}

func (BasicRPCData) Arg() interface{} {
	panic("Arg() not implemented for this type")
}

func (BasicRPCData) SeqNo() seqNumber {
	panic("SeqNo() not implemented for this type")
}

func (BasicRPCData) Err() error {
	panic("Err() not implemented for this type")
}

func (BasicRPCData) Res() interface{} {
	panic("Res() not implemented for this type")
}

func (BasicRPCData) Call() *call {
	panic("Call() not implemented for this type")
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

func (r *RPCCallData) DecodeData(l int, d decoder, p *protocolHandler, _ *callContainer) (err error) {
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

type RPCResponseData struct {
	BasicRPCData
	c     *call
	seqNo seqNumber
	err   error
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

func (r *RPCResponseData) DecodeData(l int, d decoder, _ *protocolHandler, cc *callContainer) error {
	if l != 3 {
		return errors.New("wrong message length")
	}
	if err := d.Decode(&r.seqNo); err != nil {
		return err
	}

	// Attempt to retrieve the call
	r.c = cc.retrieveCall(r.seqNo)
	if r.c == nil {
		return CallNotFoundError{r.seqNo}
	}

	// Decode the error
	var responseErr interface{}
	if err := d.Decode(responseErr); err != nil {
		return err
	}

	// Ensure the error is wrapped correctly
	if r.c.errorUnwrapper != nil {
		var dispatchErr error
		r.err, dispatchErr = r.c.errorUnwrapper.UnwrapError(responseErr)
		if dispatchErr != nil {
			return dispatchErr
		}
	} else if responseErr != nil {
		errAsString, ok := responseErr.(*string)
		if !ok {
			return errors.New("unable to convert error to string")
		}
		r.err = errors.New(*errAsString)
	}

	// Decode the result
	if r.c.res == nil {
		r.c.res = new(interface{})
	}
	return d.Decode(r.c.res)
}

func (r RPCResponseData) SeqNo() seqNumber {
	return r.seqNo
}

func (r RPCResponseData) Name() string {
	return r.c.method
}

func (r RPCResponseData) Err() error {
	return r.err
}

func (r RPCResponseData) Res() interface{} {
	return r.c.res
}

func (r RPCResponseData) Call() *call {
	return r.c
}

type RPCNotifyData struct {
	BasicRPCData
	name string
	arg  interface{}
}

func (r *RPCNotifyData) DecodeData(l int, d decoder, p *protocolHandler, _ *callContainer) (err error) {
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

type RPCCancelData struct {
	BasicRPCData
	seqno seqNumber
	name  string
}

func (r *RPCCancelData) DecodeData(l int, d decoder, p *protocolHandler, _ *callContainer) (err error) {
	if l != r.DataLength() {
		return errors.New("wrong message length")
	}
	if err = d.Decode(&r.seqno); err != nil {
		return err
	}
	return d.Decode(&r.name)
}

func (r RPCCancelData) EncodeData(v []interface{}) error {
	// TODO handle wrapErrorFunc correctly
	if len(v) != r.DataLength() {
		return errors.New("wrong message length")
	}
	v[0] = r.seqno
	v[1] = r.name
	return nil
}

func (RPCCancelData) DataLength() int {
	return 2
}

func (r RPCCancelData) SeqNo() seqNumber {
	return r.seqno
}

func (r RPCCancelData) Name() string {
	return r.name
}

type RPCCall struct {
	typ MethodType

	RPCData
}

func (c RPCCall) Type() MethodType {
	return c.typ
}

func (c RPCCall) Encode(e encoder) error {
	v := make([]interface{}, 1+c.DataLength())
	v[0] = c.typ
	if err := c.EncodeData(v[1:]); err != nil {
		return err
	}
	return e.Encode(v)
}

func (c *RPCCall) Decode(l int, d decoder, p *protocolHandler, cc *callContainer) error {
	if err := d.Decode(&c.typ); err != nil {
		return err
	}

	switch c.typ {
	case MethodCall:
		c.RPCData = &RPCCallData{}
	case MethodResponse:
		c.RPCData = &RPCResponseData{}
	case MethodNotify:
		c.RPCData = &RPCNotifyData{}
	case MethodCancel:
		c.RPCData = &RPCCancelData{}
	default:
		c.RPCData = nil
		return errors.New("invalid RPC type")
	}
	return c.DecodeData(l-1, d, p, cc)
}
