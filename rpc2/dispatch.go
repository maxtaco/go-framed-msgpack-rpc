package rpc2

import (
	"sync"
	"fmt"
)

type DecodeNext func(interface{}) error
type ServeHook func(DecodeNext) (interface{}, error)

type Dispatcher interface {
	Dispatch(m Message) error
	Call(name string, arg interface{}, res interface{}, f UnwrapErrorFunc) error
	RegisterProtocol(Protocol) error
	Reset() error
}

type ResultPair struct {
	res interface{}
	err error
}

type ClientResultPair struct {
	nxt DecodeNext
	err error
}

type Protocol struct {
	Name      string
	Methods   map[string]ServeHook
	WrapError WrapErrorFunc
}

type Dispatch struct {
	protocols map[string]Protocol
	calls     map[int]*Call
	seqid     int
	mutex     *sync.Mutex
	xp        Transporter
	log       Logger
}

func NewDispatch(xp Transporter, l Logger) *Dispatch {
	return &Dispatch{
		protocols: make(map[string]Protocol),
		calls:     make(map[int]*Call),
		seqid:     0,
		mutex:     new(sync.Mutex),
		xp:        xp,
		log:       l,
	}
}

type Request struct {
	msg       Message
	dispatch  *Dispatch
	seqno     int
	method    string
	err       interface{}
	res       interface{}
	hook      ServeHook
	wrapError WrapErrorFunc
}

type Call struct {
	ch        chan error
	seqid     int
	res       interface{}
	unwrapErr UnwrapErrorFunc
}

func NewCall(i int, res interface{}, f UnwrapErrorFunc) *Call {
	return &Call{
		ch:        make(chan error),
		seqid:     i,
		res:       res,
		unwrapErr: f,
	}
}

func (r *Request) reply() error {
	v := []interface{}{
		TYPE_RESPONSE,
		r.seqno,
		r.err,
		r.res,
	}
	return r.msg.Encode(v)
}

func (r *Request) serve() {
	prof := r.dispatch.log.StartProfiler("serve %s", r.method)
	nxt := r.msg.makeDecodeNext(func (v interface{}) {
		r.dispatch.log.ServerCall(r.seqno, r.method, v, nil)
	})

	go func() {
		res, err := r.hook(nxt)
		if prof != nil {
			prof.Stop()
		}
		r.err = r.msg.WrapError(r.wrapError, err)
		r.res = res
		err = r.reply()
		r.dispatch.log.ServerReply(r.seqno, r.method, r.err, r.res, err)
	}()
}

func (d *Dispatch) nextSeqid() int {
	d.mutex.Lock()
	ret := d.seqid
	d.seqid++
	d.mutex.Unlock()
	return ret
}

func (d *Dispatch) registerCall(seqid int, res interface{}, f UnwrapErrorFunc) *Call {
	ret := NewCall(seqid, res, f)
	d.mutex.Lock()
	d.calls[seqid] = ret
	d.mutex.Unlock()
	return ret
}

func (d *Dispatch) Call(name string, arg interface{}, res interface{}, f UnwrapErrorFunc) (err error) {

	seqid := d.nextSeqid()
	v := []interface{}{TYPE_CALL, seqid, name, arg}
	err = d.xp.Encode(v)
	if err != nil {
		return
	}
	err = <-d.registerCall(seqid, res, f).ch
	return
}

func (d *Dispatch) findServeHook(n string) (srv ServeHook, wrapError WrapErrorFunc, err error) {
	p, m := SplitMethodName(n)
	if prot, found := d.protocols[p]; !found {
		err = ProtocolNotFoundError{p}
	} else if srv, found = prot.Methods[m]; !found {
		err = MethodNotFoundError{p, m}
	} else {
		wrapError = prot.WrapError
	}
	return
}

func (d *Dispatch) dispatchCall(m Message) (err error) {
	req := Request{msg: m, dispatch: d}

	if err = m.Decode(&req.seqno); err != nil {
		return
	}
	if err = m.Decode(&req.method); err != nil {
		return
	}

	var se error
	var wrapError WrapErrorFunc
	if req.hook, wrapError, se = d.findServeHook(req.method); se != nil {
		req.err = m.WrapError(wrapError, se)
		if err = m.decodeToNull(); err != nil {
			return
		}
		d.log.ServerCall(req.seqno, res.method, nil, req.err)
		err = req.reply()
	} else {
		req.wrapError = wrapError
		req.serve()
	}
	return
}

func (d *Dispatch) RegisterProtocol(p Protocol) (err error) {
	if _, found := d.protocols[p.Name]; found {
		err = AlreadyRegisteredError{p.Name}
	} else {
		d.protocols[p.Name] = p
	}
	return err
}

func (d *Dispatch) dispatchResponse(m Message) (err error) {
	var seqno int

	if err = m.Decode(&seqno); err != nil {
		return
	}

	var call *Call
	d.mutex.Lock()
	if call = d.calls[seqno]; call != nil {
		delete(d.calls, seqno)
	}
	d.mutex.Unlock()

	if call == nil {
		d.log.CallUnexpectedReply(seqno)
		err = m.decodeToNull()
		return
	}

	var apperr error

	if apperr, err = m.DecodeError(call.unwrapErr); err == nil {
		decode_to := call.res
		if decode_to == nil {
			var tmp interface{}
			decode_to = &tmp
		}
		err = m.Decode(decode_to)
	}

	if err != nil {
		m.decodeToNull()
		if apperr == nil {
			apperr = err
		}
	}

	call.ch <- apperr

	return
}

func (d *Dispatch) Reset() error {
	d.mutex.Lock()
	for k, v := range d.calls {
		v.ch <- EofError{}
		delete(d.calls, k)
	}
	d.mutex.Unlock()
	return nil
}

func (d *Dispatch) Dispatch(m Message) (err error) {
	if m.nFields == 4 {
		err = d.dispatchQuad(m)
	} else {
		err = NewDispatcherError("can only handle message quads (got n=%d fields)", m.nFields)
	}
	return
}

func (d *Dispatch) dispatchQuad(m Message) (err error) {
	var l int
	if err = m.Decode(&l); err != nil {
		return
	}

	switch l {
	case TYPE_CALL:
		d.dispatchCall(m)
	case TYPE_RESPONSE:
		d.dispatchResponse(m)
	default:
		err = NewDispatcherError("Unexpected message type=%d; wanted CALL=%d or RESPONSE=%d",
			l, TYPE_CALL, TYPE_RESPONSE)
	}
	return
}
