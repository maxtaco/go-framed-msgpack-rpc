package rpc2

type Dispatcher interface {
	Dispatch(m Message) error
}

type ResultPair struct {
	res interface{}
	err error
}

type DecodeNext func(interface{}) error
type ServeHook func(DecodeNext) (interface{}, error)

type Method struct {
	hook ServeHook
}

type Dispatch struct {
	methods map[string]Method
	calls   map[int]Call
}

type Request struct {
	message  Message
	dispatch *Dispatch
	seqno    int
	err      interface{}
	res      interface{}
	method   Method
}

type Call struct {
}

func (r *Request) reply() error {
	v := []interface{}{
		TYPE_RESPONSE,
		r.seqno,
		r.err,
		r.res,
	}
	return r.message.Encode(v)
}

func (r *Request) serve() {
	ch := make(chan ResultPair)

	go func() {
		decode := func(i interface{}) error {
			return r.message.Decode(i)
		}
		res, err := r.method.hook(decode)
		ch <- ResultPair{res, err}

	}()

	rp := <-ch
	r.err = r.message.WrapError(rp.err)
	r.res = rp.res
	return
}

func (d *Dispatch) dispatchInvoke(m Message) (err error) {
	var name string
	req := Request{message: m, dispatch: d}

	if err = m.Decode(&req.seqno); err != nil {
		return
	}
	if err = m.Decode(&name); err != nil {
		return
	}

	var found bool

	if req.method, found = d.methods[name]; !found {
		se := MethodNotFoundError{name}
		req.err = m.WrapError(se)
		m.decodeToNull()
	} else {
		req.serve()
	}

	return req.reply()
}

func (d *Dispatch) dispatchResponse(m Message) (err error) {
	return
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
	case TYPE_INVOKE:
		d.dispatchInvoke(m)
	case TYPE_RESPONSE:
		d.dispatchResponse(m)
	default:
		err = NewDispatcherError("Unexpected message type=%d; wanted INVOKE=%d or RESPONSE=%d",
			l, TYPE_INVOKE, TYPE_RESPONSE)
	}
	return
}
