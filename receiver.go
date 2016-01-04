package rpc

import (
	"sync"

	"golang.org/x/net/context"
)

type messageHandler func(RPCMessage) error

type task struct {
	seqid      seqNumber
	cancelFunc context.CancelFunc
}

type receiver interface {
	Receive(RPCMessage) error
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

	log             LogInterface
	messageHandlers map[MethodType]messageHandler
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
	r.messageHandlers = map[MethodType]messageHandler{
		MethodNotify:   r.receiveNotify,
		MethodCall:     r.receiveCall,
		MethodResponse: r.receiveResponse,
		MethodCancel:   r.receiveCancel,
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

func (d *receiveHandler) Receive(rpc RPCMessage) error {
	handler, ok := d.messageHandlers[rpc.Type()]
	if !ok {
		return NewDispatcherError("invalid message type")
	}
	return handler(rpc)
}

func (r *receiveHandler) receiveNotify(rpc RPCMessage) error {
	req := newRequest(rpc, r.log)
	return r.handleReceiveDispatch(req)
}

func (r *receiveHandler) receiveCall(rpc RPCMessage) error {
	req := newRequest(rpc, r.log)
	return r.handleReceiveDispatch(req)
}

func (r *receiveHandler) receiveCancel(rpc RPCMessage) error {
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
	req.Serve(r.writer, serveHandler, wrapErrorFunc)
	return nil
}

func (r *receiveHandler) receiveResponse(rpc RPCMessage) (err error) {
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
