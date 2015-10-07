package rpc

import (
	"errors"
	"sync"
)

type DecodeNext func(interface{}) error
type ServeHook func(DecodeNext) (interface{}, error)
type ServeNotifyHook func(DecodeNext) error

type dispatcher interface {
	Call(name string, arg interface{}, res interface{}, f UnwrapErrorFunc) error
	Notify(name string, arg interface{}) error
	RegisterProtocol(Protocol) error
	Dispatch(m *message) error
	Reset() error
}

type Protocol struct {
	Name          string
	Methods       map[string]ServeHook
	NotifyMethods map[string]ServeNotifyHook
	WrapError     WrapErrorFunc
}

type dispatch struct {
	enc             encoder
	dec             byteReadingDecoder
	protocols       map[string]Protocol
	calls           map[int]*call
	seqid           int
	callsMutex      *sync.Mutex
	writeCh         chan []byte
	errCh           chan error
	log             LogInterface
	wrapErrorFunc   WrapErrorFunc
	messageHandlers map[int]messageHandler
}

type messageHandler struct {
	dispatchFunc  func(*message) error
	messageLength int
}

func newDispatch(enc encoder, dec byteReadingDecoder, l LogInterface, wef WrapErrorFunc) *dispatch {
	d := &dispatch{
		enc:           enc,
		dec:           dec,
		protocols:     make(map[string]Protocol),
		calls:         make(map[int]*call),
		seqid:         0,
		callsMutex:    new(sync.Mutex),
		log:           l,
		wrapErrorFunc: wef,
	}
	d.messageHandlers = map[int]messageHandler{
		TYPE_NOTIFY:   {dispatchFunc: d.dispatchNotify, messageLength: 3},
		TYPE_CALL:     {dispatchFunc: d.dispatchCall, messageLength: 4},
		TYPE_RESPONSE: {dispatchFunc: d.dispatchResponse, messageLength: 4},
	}
	return d
}

type request struct {
	msg           *message
	dispatch      *dispatch
	seqno         int
	method        string
	err           interface{}
	res           interface{}
	hook          ServeHook
	wrapErrorFunc WrapErrorFunc
}

type notifyRequest struct {
	msg           *message
	dispatch      *dispatch
	method        string
	err           interface{}
	hook          ServeNotifyHook
	wrapErrorFunc WrapErrorFunc
}

type call struct {
	ch              chan error
	method          string
	seqid           int
	res             interface{}
	unwrapErrorFunc UnwrapErrorFunc
	profiler        Profiler
}

func (c *call) Init() {
	c.ch = make(chan error)
}

func (r *request) reply() error {
	v := []interface{}{
		TYPE_RESPONSE,
		r.seqno,
		r.err,
		r.res,
	}
	return r.dispatch.enc.Encode(v)
}

func (r *request) serve() {
	prof := r.dispatch.log.StartProfiler("serve %s", r.method)
	nxt := r.msg.makeDecodeNext(func(v interface{}) {
		r.dispatch.log.ServerCall(r.seqno, r.method, nil, v)
	})

	go func() {
		res, err := r.hook(nxt)
		if prof != nil {
			prof.Stop()
		}
		r.err = wrapError(r.wrapErrorFunc, err)
		r.res = res
		r.dispatch.log.ServerReply(r.seqno, r.method, err, r.res)
		if err = r.reply(); err != nil {
			r.dispatch.log.Warning("Reply error for %d: %s", r.seqno, err.Error())
		}
	}()
}

func (r *notifyRequest) serve() {
	prof := r.dispatch.log.StartProfiler("serve %s", r.method)
	nxt := r.msg.makeDecodeNext(func(v interface{}) {
		r.dispatch.log.ServerNotifyCall(r.method, nil, v)
	})

	go func() {
		err := r.hook(nxt)
		if prof != nil {
			prof.Stop()
		}
		r.dispatch.log.ServerNotifyComplete(r.method, err)
	}()
}

func (d *dispatch) nextSeqid() int {
	ret := d.seqid
	d.seqid++
	return ret
}

func (d *dispatch) registerCall(c *call) {
	d.calls[c.seqid] = c
}

func (d *dispatch) Call(name string, arg interface{}, res interface{}, f UnwrapErrorFunc) (err error) {

	d.callsMutex.Lock()

	seqid := d.nextSeqid()
	v := []interface{}{TYPE_CALL, seqid, name, arg}
	profiler := d.log.StartProfiler("call %s", name)
	call := &call{
		method:          name,
		seqid:           seqid,
		res:             res,
		unwrapErrorFunc: f,
		profiler:        profiler,
	}
	call.Init()
	d.registerCall(call)

	d.callsMutex.Unlock()

	err = d.enc.Encode(v)
	if err != nil {
		return err
	}
	d.log.ClientCall(seqid, name, arg)
	err = <-call.ch
	return
}

func (d *dispatch) Notify(name string, arg interface{}) (err error) {

	v := []interface{}{TYPE_NOTIFY, name, arg}
	err = d.enc.Encode(v)
	if err != nil {
		return
	}
	d.log.ClientNotify(name, arg)
	return
}

func (d *dispatch) readDecoded(v interface{}) error {
	return nil
}

func (d *dispatch) findServeHook(n string) (srv ServeHook, wrapErrorFunc WrapErrorFunc, err error) {
	p, m := SplitMethodName(n)
	var prot Protocol
	var found bool
	if prot, found = d.protocols[p]; !found {
		err = ProtocolNotFoundError{p}
	} else if srv, found = prot.Methods[m]; !found {
		err = MethodNotFoundError{p, m}
	}
	if found {
		wrapErrorFunc = prot.WrapError
	}
	if wrapErrorFunc == nil {
		wrapErrorFunc = d.wrapErrorFunc
	}
	return
}

func (d *dispatch) findServeNotifyHook(n string) (srv ServeNotifyHook, wrapErrorFunc WrapErrorFunc, err error) {
	p, m := SplitMethodName(n)
	var prot Protocol
	var found bool
	if prot, found = d.protocols[p]; !found {
		err = ProtocolNotFoundError{p}
	} else if srv, found = prot.NotifyMethods[m]; !found {
		err = MethodNotFoundError{p, m}
	}
	if found {
		wrapErrorFunc = prot.WrapError
	}
	if wrapErrorFunc == nil {
		wrapErrorFunc = d.wrapErrorFunc
	}
	return
}

func (d *dispatch) RegisterProtocol(p Protocol) (err error) {
	if _, found := d.protocols[p.Name]; found {
		err = AlreadyRegisteredError{p.Name}
	} else {
		d.protocols[p.Name] = p
	}
	return err
}

func (d *dispatch) Reset() error {
	d.callsMutex.Lock()
	for k, v := range d.calls {
		v.ch <- EofError{}
		delete(d.calls, k)
	}
	d.callsMutex.Unlock()
	return nil
}

func (d *dispatch) Dispatch(m *message) error {
	var l int
	if err := d.decodeMessage(m, &l); err != nil {
		return err
	}
	handler, ok := d.messageHandlers[l]
	if !ok {
		return NewDispatcherError("invalid message type")
	}
	if m.nFields != handler.messageLength {
		return NewDispatcherError("wrong number of fields for message (got n=%d, expected n=%d)", m.nFields, handler.messageLength)

	}
	return handler.dispatchFunc(m)
}

func (d *dispatch) dispatchNotify(m *message) (err error) {
	req := notifyRequest{msg: m, dispatch: d}

	if err = d.decodeMessage(m, &req.method); err != nil {
		return
	}

	var se error
	var wrapErrorFunc WrapErrorFunc
	if req.hook, wrapErrorFunc, se = d.findServeNotifyHook(req.method); se != nil {
		req.err = wrapError(wrapErrorFunc, se)
		if err = d.decodeToNull(m); err != nil {
			return
		}
		d.log.ServerNotifyCall(req.method, se, nil)
	} else {
		req.wrapErrorFunc = wrapErrorFunc
		req.serve()
	}
	return
}

func (d *dispatch) dispatchCall(m *message) (err error) {
	req := request{msg: m, dispatch: d}

	if err = d.decodeMessage(m, &req.seqno); err != nil {
		return
	}
	if err = d.decodeMessage(m, &req.method); err != nil {
		return
	}

	var se error
	var wrapErrorFunc WrapErrorFunc
	if req.hook, wrapErrorFunc, se = d.findServeHook(req.method); se != nil {
		req.err = wrapError(wrapErrorFunc, se)
		if err = d.decodeToNull(m); err != nil {
			return
		}
		d.log.ServerCall(req.seqno, req.method, se, nil)
		err = req.reply()
	} else {
		req.wrapErrorFunc = wrapErrorFunc
		req.serve()
	}
	return
}

// Server
func (d *dispatch) dispatchResponse(m *message) (err error) {
	var seqno int

	if err = d.decodeMessage(m, &seqno); err != nil {
		return
	}

	d.callsMutex.Lock()
	var call *call
	if call = d.calls[seqno]; call != nil {
		delete(d.calls, seqno)
	}
	d.callsMutex.Unlock()

	if call == nil {
		d.log.UnexpectedReply(seqno)
		err = d.decodeToNull(m)
		return
	}

	var apperr error

	if call.profiler != nil {
		call.profiler.Stop()
	}

	if apperr, err = d.decodeError(m, call.unwrapErrorFunc); err == nil {
		decode_to := call.res
		if decode_to == nil {
			var tmp interface{}
			decode_to = &tmp
		}
		err = d.decodeMessage(m, decode_to)
		d.log.ClientReply(seqno, call.method, err, decode_to)
	} else {
		d.log.ClientReply(seqno, call.method, err, nil)
	}

	if err != nil {
		d.decodeToNull(m)
		if apperr == nil {
			apperr = err
		}
	}

	call.ch <- apperr

	return
}

func (d *dispatch) decodeMessage(m *message, i interface{}) error {
	err := d.dec.Decode(i)
	if err == nil {
		m.nDecoded++
	}
	return err
}

func (d *dispatch) decodeToNull(m *message) error {
	var err error
	for err == nil && m.nDecoded < m.nFields {
		var i interface{}
		d.decodeMessage(m, &i)
	}
	return err
}

func (d *dispatch) decodeError(m *message, f UnwrapErrorFunc) (app error, dispatch error) {
	var s string
	if f != nil {
		app, dispatch = f(m.makeDecodeNext(nil))
	} else if dispatch = d.decodeMessage(m, &s); dispatch == nil && len(s) > 0 {
		app = errors.New(s)
	}
	return
}

func wrapError(f WrapErrorFunc, e error) interface{} {
	if f != nil {
		return f(e)
	} else if e == nil {
		return nil
	} else {
		return e.Error()
	}
}
