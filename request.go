package rpc

import (
	"fmt"

	"golang.org/x/net/context"
)

type request interface {
	RPC
	CancelFunc() context.CancelFunc
	Reply(encoder, LogInterface) error
	Serve(encoder, *ServeHandlerDescription, WrapErrorFunc, LogInterface)
	LogInvocation(log LogInterface, err error, arg interface{})
	LogCompletion(log LogInterface, err error)
}

type requestImpl struct {
	*RPCCall
	ctx        context.Context
	cancelFunc context.CancelFunc
}

func (req *requestImpl) CancelFunc() context.CancelFunc {
	return req.cancelFunc
}

func (r *requestImpl) LogInvocation(LogInterface, error, interface{}) {}
func (r *requestImpl) LogCompletion(LogInterface, error)              {}
func (r *requestImpl) Reply(encoder, LogInterface) error              { return nil }
func (r *requestImpl) Serve(byteReadingDecoder, encoder, *ServeHandlerDescription, WrapErrorFunc, LogInterface) {
}

type callRequest struct {
	requestImpl
}

func newCallRequest(rpc *RPCCall) *callRequest {
	ctx, cancel := context.WithCancel(context.Background())
	return &callRequest{
		requestImpl: requestImpl{
			RPCCall:    rpc,
			ctx:        ctx,
			cancelFunc: cancel,
		},
	}
}

func (r *callRequest) LogInvocation(log LogInterface, err error, arg interface{}) {
	log.ServerCall(r.seqno, r.method, err, arg)
}

func (r *callRequest) LogCompletion(log LogInterface, err error) {
	log.ServerReply(r.seqno, r.method, err, r.res)
}

func (r *callRequest) Reply(enc encoder, log LogInterface) error {
	var err error
	select {
	case <-r.ctx.Done():
		// TODO: Use newCanceledError and log.Info:
		// https://github.com/keybase/go-framed-msgpack-rpc/issues/29
		// .
		err = fmt.Errorf("call canceled for seqno %d", r.SeqNo())
		log.Warning(err.Error())
	default:
		v := []interface{}{
			MethodResponse,
			r.seqno,
			r.err,
			r.res,
		}
		err = enc.Encode(v)
		if err != nil {
			log.Warning("Reply error for %d: %s", r.SeqNo(), err.Error())
		}
	}
	return err
}

func (r *callRequest) Serve(transmitter encoder, handler *ServeHandlerDescription, wrapErrorFunc WrapErrorFunc, log LogInterface) {

	prof := log.StartProfiler("serve %s", r.method)
	arg := r.Arg()

	go func() {
		r.LogInvocation(log, err, arg)
		if err != nil {
			r.err = wrapError(wrapErrorFunc, err)
		} else {
			res, err := handler.Handler(r.ctx, arg)
			r.err = wrapError(wrapErrorFunc, err)
			r.res = res
		}
		prof.Stop()
		r.LogCompletion(log, err)
		r.Reply(transmitter, log)
	}()
}

type notifyRequest struct {
	requestImpl
}

func newNotifyRequest(rpc *RPCCall) *notifyRequest {
	ctx, cancel := context.WithCancel(context.Background())
	return &notifyRequest{
		requestImpl: requestImpl{
			RPCCall:    rpc,
			ctx:        ctx,
			cancelFunc: cancel,
		},
	}
}

func (r *notifyRequest) LogInvocation(log LogInterface, err error, arg interface{}) {
	log.ServerNotifyCall(r.method, err, arg)
}

func (r *notifyRequest) LogCompletion(log LogInterface, err error) {
	log.ServerNotifyComplete(r.method, err)
}

func (r *notifyRequest) Serve(transmitter encoder, handler *ServeHandlerDescription, wrapErrorFunc WrapErrorFunc, log LogInterface) {

	prof := log.StartProfiler("serve-notify %s", r.method)
	arg := r.Arg()

	go func() {
		r.LogInvocation(log, err, arg)
		if err == nil {
			_, err = handler.Handler(r.ctx, arg)
		}
		prof.Stop()
		r.LogCompletion(log, err)
	}()
}

type cancelRequest struct {
	requestImpl
}

func newCancelRequest(rpc *RPCCall) *cancelRequest {
	return &cancelRequest{
		requestImpl: requestImpl{
			RPCCall: rpc,
			ctx:     context.Background(),
		},
	}
}

func (r *cancelRequest) LogInvocation(log LogInterface, err error, arg interface{}) {
	log.ServerCancelCall(r.seqno, r.method)
}

func newRequest(rpc *RPCCall) request {
	switch rpc.Type() {
	case MethodCall:
		return newCallRequest(rpc)
	case MethodNotify:
		return newNotifyRequest(rpc)
	case MethodCancel:
		return newCancelRequest(rpc)
	}
	return nil
}
