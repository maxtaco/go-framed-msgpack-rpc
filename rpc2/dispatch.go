package rpc2

import (
	"fmt"
	"sync"
)

type DecodeNext func(interface{}) error
type ServeHook func(DecodeNext) (interface{}, error)
type WarnFunc func(string)

type Dispatcher interface {
	Dispatch(m Message) error
	Warn(string)
	Call(name string, arg interface{}) (ret DecodeNext, err error)
	RegisterProtocol(Protocol) error
	Reset() error
}

type ResultPair struct {
	res interface{}
	err error
}

type MessagePair struct {
	msg Message
	err error
}

type Protocol struct {
	Name    string
	Methods map[string]ServeHook
}

type Dispatch struct {
	protocols map[string]Protocol
	calls     map[int]*Call
	seqid     int
	mutex     *sync.Mutex
	xp        Transporter
	warnFn    func(string)
}

func NewDispatch(xp Transporter, w WarnFunc) *Dispatch {
	return &Dispatch{
		protocols: make(map[string]Protocol),
		calls:     make(map[int]*Call),
		seqid:     0,
		mutex:     new(sync.Mutex),
		xp:        xp,
		warnFn:    w,
	}
}

type Request struct {
	msg      Message
	dispatch *Dispatch
	seqno    int
	err      interface{}
	res      interface{}
	hook     ServeHook
}

type Call struct {
	ch    chan MessagePair
	seqid int
}

func NewCall(i int) *Call {
	return &Call{
		ch:    make(chan MessagePair),
		seqid: i,
	}
}

func (r *Request) reply() error {
	v := []interface{}{
		TYPE_RESPONSE,
		r.seqno,
		r.err,
		r.res,
	}
	fmt.Printf("doing reply ---> %v\n", v)
	return r.msg.Encode(v)
}

func (r *Request) serve() {
	ch := make(chan ResultPair)

	go func() {
		res, err := r.hook(r.msg.makeDecodeNext())
		ch <- ResultPair{res, err}

	}()

	rp := <-ch
	r.err = r.msg.WrapError(rp.err)
	r.res = rp.res
	return
}

func (d *Dispatch) nextSeqid() int {
	d.mutex.Lock()
	ret := d.seqid
	d.seqid++
	d.mutex.Unlock()
	return ret
}

func (d *Dispatch) registerCall(seqid int) *Call {
	ret := NewCall(seqid)
	d.mutex.Lock()
	d.calls[seqid] = ret
	d.mutex.Unlock()
	return ret
}

func (d *Dispatch) Call(name string, arg interface{}) (ret DecodeNext, err error) {

	seqid := d.nextSeqid()
	v := []interface{}{TYPE_CALL, seqid, name, arg}
	err = d.xp.Encode(v)
	if err != nil {
		return
	}
	mp := <-d.registerCall(seqid).ch
	if err = mp.err; err == nil {
		ret = mp.msg.makeDecodeNext()
	}
	return
}

func (d *Dispatch) findServeHook(n string) (srv ServeHook, err error) {
	p, m := SplitMethodName(n)
	if prot, found := d.protocols[p]; !found {
		err = ProtocolNotFoundError{p}
	} else if srv, found = prot.Methods[m]; !found {
		err = MethodNotFoundError{p, m}
	}
	return
}

func (d *Dispatch) dispatchCall(m Message) (err error) {
	var name string
	req := Request{msg: m, dispatch: d}

	if err = m.Decode(&req.seqno); err != nil {
		return
	}
	if err = m.Decode(&name); err != nil {
		return
	}

	var se error
	if req.hook, se = d.findServeHook(name); se != nil {
		req.err = m.WrapError(se)
		if err = m.decodeToNull(); err != nil {
			return
		}
	} else {
		req.serve()
	}

	return req.reply()
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
		d.Warn(fmt.Sprintf("Unexpected call; no sequence ID for %d", seqno))
		err = m.decodeToNull()
		return
	}

	mp := MessagePair{msg: m}

	if mp.err, err = m.DecodeError(); err != nil {
		m.decodeToNull()
		return
	}

	call.ch <- mp

	return
}

func (d *Dispatch) Reset() error {
	d.mutex.Lock()
	for k, v := range d.calls {
		v.ch <- MessagePair{err: EofError{}}
		delete(d.calls, k)
	}
	d.mutex.Unlock()
	return nil
}

func (d *Dispatch) Warn(s string) {
	d.warnFn(s)
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
