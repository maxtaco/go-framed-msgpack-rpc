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
	RegisterProtocol(Protocol) error
	Close(err error) chan struct{}
	AddCloseListener(chan error)
}

type callRetrieval struct {
	seqid seqNumber
	ch    chan *call
}

type receiveHandler struct {
	writer encoder

	protocols     map[string]Protocol
	wrapErrorFunc WrapErrorFunc
	tasks         map[int]context.CancelFunc

	listenerMtx sync.Mutex
	listeners   map[chan<- error]struct{}

	// Stops all loops when closed
	stopCh chan struct{}
	// Closed once all loops are finished
	closedCh chan struct{}

	rmCallCh chan callRetrieval

	// Task loop channels
	taskBeginCh  chan *task
	taskCancelCh chan seqNumber
	taskEndCh    chan seqNumber

	log             LogInterface
	messageHandlers map[MethodType]messageHandler
}

func newReceiveHandler(enc encoder, rmCallCh chan callRetrieval, l LogInterface, wef WrapErrorFunc) *receiveHandler {
	r := &receiveHandler{
		writer:    enc,
		protocols: make(map[string]Protocol),
		tasks:     make(map[int]context.CancelFunc),
		listeners: make(map[chan<- error]struct{}),
		rmCallCh:  rmCallCh,
		stopCh:    make(chan struct{}),
		closedCh:  make(chan struct{}),

		taskBeginCh:  make(chan *task),
		taskCancelCh: make(chan seqNumber),
		taskEndCh:    make(chan seqNumber),

		log:           l,
		wrapErrorFunc: wef,
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
	req := newRequest(rpc)
	return r.handleReceiveDispatch(req)
}

func (r *receiveHandler) receiveCall(rpc *RPCCall) error {
	req := newRequest(rpc)
	return r.handleReceiveDispatch(req)
}

func (r *receiveHandler) receiveCancel(rpc *RPCCall) error {
	req := newRequest(rpc)
	req.LogInvocation(r.log, nil, nil)
	r.taskCancelCh <- rpc.SeqNo()
	return nil
}

func (r *receiveHandler) handleReceiveDispatch(req request) error {
	serveHandler, wrapErrorFunc, se := r.findServeHandler(req.Name())
	if se != nil {
		req.LogInvocation(r.log, se, nil)
		return req.Reply(r.writer, r.log)
	}
	r.taskBeginCh <- &task{req.SeqNo(), req.CancelFunc()}
	req.Serve(r.writer, serveHandler, wrapErrorFunc, r.log)
	return nil
}

func (r *receiveHandler) receiveResponse(rpc *RPCCall) (err error) {
	ch := make(chan *call)
	r.rmCallCh <- callRetrieval{rpc.SeqNo(), ch}
	call := <-ch

	if call == nil {
		r.log.UnexpectedReply(rpc.SeqNo())
		return CallNotFoundError{rpc.SeqNo()}
	}

	var apperr error

	call.profiler.Stop()

	if apperr, err = decodeError(r.reader, m, call.errorUnwrapper); err == nil {
		decodeTo := call.res
		if decodeTo == nil {
			decodeTo = new(interface{})
		}
		err = decodeMessage(r.reader, m, decodeTo)
		r.log.ClientReply(m.seqno, call.method, err, decodeTo)
	} else {
		r.log.ClientReply(m.seqno, call.method, err, nil)
	}

	if err != nil {
		decodeToNull(r.reader, m)
		if apperr == nil {
			apperr = err
		}
	}

	_ = call.Finish(apperr)

	return
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
