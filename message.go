package rpc

import (
	"errors"
)

type RPCData interface {
	Type() MethodType
	Name() string
	SeqNo() seqNumber
	MinLength() int
	DecodeData(int, decoder, *protocolHandler, *callContainer) error
}

type RPCCallData struct {
	seqno seqNumber
	name  string
	arg   interface{}
}

func (RPCCallData) MinLength() int {
	return 3
}

func (r *RPCCallData) DecodeData(l int, d decoder, p *protocolHandler, _ *callContainer) (err error) {
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

func (r RPCCallData) Type() MethodType {
	return MethodCall
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
	c   *call
	err error
}

func (r RPCResponseData) MinLength() int {
	return 3
}

func (r *RPCResponseData) DecodeData(l int, d decoder, _ *protocolHandler, cc *callContainer) error {
	var seqNo seqNumber
	if err := d.Decode(&seqNo); err != nil {
		return err
	}

	// Attempt to retrieve the call
	r.c = cc.RetrieveCall(seqNo)
	if r.c == nil {
		return CallNotFoundError{seqNo}
	}

	// Decode the error
	var responseErr interface{}
	if r.c.errorUnwrapper != nil {
		responseErr = r.c.errorUnwrapper.MakeArg()
	}
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
		return NilResultError{seqNo}
	}
	return d.Decode(r.c.res)
}

func (r RPCResponseData) Type() MethodType {
	return MethodResponse
}

func (r RPCResponseData) SeqNo() seqNumber {
	return r.c.seqid
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

func (r RPCResponseData) ResponseCh() chan *RPCResponseData {
	return r.c.resultCh
}

type RPCNotifyData struct {
	name string
	arg  interface{}
}

func (r *RPCNotifyData) DecodeData(l int, d decoder, p *protocolHandler, _ *callContainer) (err error) {
	if err = d.Decode(&r.name); err != nil {
		return err
	}
	if r.arg, err = p.getArg(r.name); err != nil {
		return err
	}
	return d.Decode(r.arg)
}

func (RPCNotifyData) MinLength() int {
	return 2
}

func (r RPCNotifyData) Type() MethodType {
	return MethodNotify
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
	seqno seqNumber
	name  string
}

func (r *RPCCancelData) DecodeData(l int, d decoder, p *protocolHandler, _ *callContainer) (err error) {
	if err = d.Decode(&r.seqno); err != nil {
		return err
	}
	return d.Decode(&r.name)
}

func (RPCCancelData) MinLength() int {
	return 2
}

func (r RPCCancelData) Type() MethodType {
	return MethodCancel
}

func (r RPCCancelData) SeqNo() seqNumber {
	return r.seqno
}

func (r RPCCancelData) Name() string {
	return r.name
}

func DecodeRPC(l int, d decoder, p *protocolHandler, cc *callContainer) (RPCData, error) {
	var typ MethodType
	if err := d.Decode(&typ); err != nil {
		return nil, err
	}

	var data RPCData
	switch typ {
	case MethodCall:
		data = &RPCCallData{}
	case MethodResponse:
		data = &RPCResponseData{}
	case MethodNotify:
		data = &RPCNotifyData{}
	case MethodCancel:
		data = &RPCCancelData{}
	default:
		return nil, errors.New("invalid RPC type")
	}
	l--
	if l < data.MinLength() {
		return nil, errors.New("wrong message length")
	}
	if err := data.DecodeData(l, d, p, cc); err != nil {
		return nil, err
	}
	return data, nil
}
