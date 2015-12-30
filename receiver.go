package rpc

import (
	"sync"

	"golang.org/x/net/context"
)

type messageHandler func(*RPCCall) error

type task struct {
	seqid      seqNumber
	cancelFunc context.CancelFunc
}

type receiver interface {
	Receive(*RPCCall) error
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

	calls    *callContainer
	rmCallCh chan callRetrieval

	// Task loop channels
	taskBeginCh  chan *task
	taskCancelCh chan seqNumber
	taskEndCh    chan seqNumber

	log             LogInterface
	messageHandlers map[MethodType]messageHandler
}

func newReceiveHandler(enc encoder, protHandler *protocolHandler, calls *callContainer, l LogInterface) *receiveHandler {
	r := &receiveHandler{
		writer:      enc,
		protHandler: protHandler,
		tasks:       make(map[int]context.CancelFunc),
		listeners:   make(map[chan<- error]struct{}),
		calls:       calls,
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

func (d *receiveHandler) Receive(rpc *RPCCall) error {
	handler, ok := d.messageHandlers[rpc.Type()]
	if !ok {
		return NewDispatcherError("invalid message type")
	}
	return handler(rpc)
}

func (r *receiveHandler) receiveNotify(rpc *RPCCall) error {
	req := newRequest(rpc, r.log)
	return r.handleReceiveDispatch(req)
}

func (r *receiveHandler) receiveCall(rpc *RPCCall) error {
	req := newRequest(rpc, r.log)
	return r.handleReceiveDispatch(req)
}

func (r *receiveHandler) receiveCancel(rpc *RPCCall) error {
	r.log.ServerCancelCall(rpc.SeqNo(), rpc.Name())
	r.taskCancelCh <- rpc.SeqNo()
	return nil
}

func (r *receiveHandler) handleReceiveDispatch(req request) error {
	serveHandler, wrapErrorFunc, se := r.protHandler.findServeHandler(req.Name())
	if se != nil {
		req.LogInvocation(se)
		return req.Reply(r.writer, nil, se)
	}
	r.taskBeginCh <- &task{req.SeqNo(), req.CancelFunc()}
	req.Serve(r.writer, serveHandler, wrapErrorFunc)
	return nil
}

func (r *receiveHandler) receiveResponse(rpc *RPCCall) (err error) {
	call := rpc.Call()

	if call == nil {
		r.log.UnexpectedReply(rpc.SeqNo())
		return CallNotFoundError{rpc.SeqNo()}
	}

	_ = call.Finish(rpc.Err())
	call.profiler.Stop()
	r.log.ClientReply(rpc.SeqNo(), rpc.Name(), rpc.Err(), rpc.Res())

	return rpc.Err()
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
