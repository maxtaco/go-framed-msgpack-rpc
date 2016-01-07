package rpc

import (
	"sync"

	"golang.org/x/net/context"
)

type task struct {
	seqid      seqNumber
	cancelFunc context.CancelFunc
}

type receiver interface {
	Receive(RPCData) error
	Close(err error) chan struct{}
	AddCloseListener(chan<- error)
}

type receiveHandler struct {
	writer      encoder
	protHandler *protocolHandler

	tasks map[int]context.CancelFunc

	listenerMtx sync.Mutex
	listeners   map[chan<- error]struct{}

	// Stops all loops when closed
	stopCh chan struct{}
	// Closed once all loops are finished
	closedCh chan struct{}

	// Task loop channels
	taskBeginCh  chan *task
	taskCancelCh chan seqNumber
	taskEndCh    chan seqNumber

	log LogInterface
}

func newReceiveHandler(enc encoder, protHandler *protocolHandler, l LogInterface) *receiveHandler {
	r := &receiveHandler{
		writer:      enc,
		protHandler: protHandler,
		tasks:       make(map[int]context.CancelFunc),
		listeners:   make(map[chan<- error]struct{}),
		stopCh:      make(chan struct{}),
		closedCh:    make(chan struct{}),

		taskBeginCh:  make(chan *task),
		taskCancelCh: make(chan seqNumber),
		taskEndCh:    make(chan seqNumber),

		log: l,
	}
	go r.taskLoop()
	return r
}

func (r *receiveHandler) taskLoop() {
	tasks := make(map[seqNumber]context.CancelFunc)
	for {
		select {
		case <-r.stopCh:
			close(r.closedCh)
			return
		case t := <-r.taskBeginCh:
			tasks[t.seqid] = t.cancelFunc
		case seqid := <-r.taskCancelCh:
			if cancelFunc, ok := tasks[seqid]; ok {
				cancelFunc()
			}
			delete(tasks, seqid)
		case seqid := <-r.taskEndCh:
			delete(tasks, seqid)
		}
	}
}

func (r *receiveHandler) Receive(rpc RPCData) error {
	switch rpc.Type() {
	case MethodNotify:
		if rpcData, ok := rpc.(*RPCNotifyData); ok {
			return r.receiveNotify(rpcData)
		}
	case MethodCall:
		if rpcData, ok := rpc.(*RPCCallData); ok {
			return r.receiveCall(rpcData)
		}
	case MethodResponse:
		if rpcData, ok := rpc.(*RPCResponseData); ok {
			return r.receiveResponse(rpcData)
		}
	case MethodCancel:
		if rpcData, ok := rpc.(*RPCCancelData); ok {
			return r.receiveCancel(rpcData)
		}
	default:
	}
	return NewDispatcherError("invalid message type")
}

func (r *receiveHandler) receiveNotify(rpc *RPCNotifyData) error {
	req := newNotifyRequest(rpc, r.log)
	return r.handleReceiveDispatch(req)
}

func (r *receiveHandler) receiveCall(rpc *RPCCallData) error {
	req := newCallRequest(rpc, r.log)
	return r.handleReceiveDispatch(req)
}

func (r *receiveHandler) receiveCancel(rpc *RPCCancelData) error {
	r.log.ServerCancelCall(rpc.SeqNo(), rpc.Name())
	r.taskCancelCh <- rpc.SeqNo()
	return nil
}

func (r *receiveHandler) handleReceiveDispatch(req request) error {
	serveHandler, wrapErrorFunc, se := r.protHandler.findServeHandler(req.Name())
	if se != nil {
		req.LogInvocation(se)
		return req.Reply(r.writer, nil, wrapError(wrapErrorFunc, se))
	}
	r.taskBeginCh <- &task{req.SeqNo(), req.CancelFunc()}
	go req.Serve(r.writer, serveHandler, wrapErrorFunc)
	return nil
}

func (r *receiveHandler) receiveResponse(rpc *RPCResponseData) (err error) {
	callResponseCh := rpc.ResponseCh()

	if callResponseCh == nil {
		r.log.UnexpectedReply(rpc.SeqNo())
		return CallNotFoundError{rpc.SeqNo()}
	}

	callResponseCh <- rpc
	return nil
}

func (r *receiveHandler) Close(err error) chan struct{} {
	close(r.stopCh)
	r.broadcast(err)
	return r.closedCh
}

func (r *receiveHandler) AddCloseListener(ch chan<- error) {
	r.listenerMtx.Lock()
	defer r.listenerMtx.Unlock()
	r.listeners[ch] = struct{}{}
}

func (r *receiveHandler) broadcast(err error) {
	r.listenerMtx.Lock()
	defer r.listenerMtx.Unlock()
	for ch := range r.listeners {
		select {
		case ch <- err:
		default:
		}
	}
}
