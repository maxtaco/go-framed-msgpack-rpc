package rpc2

type Dispatcher interface {
	Dispatch(msg Message) error
	DispatchTriple(t Transporter) error
	DispatchQuad(t Transporter) error
}

type Dispatch struct {
	notifies map[string]Notify
	methods  map[string]Method
	calls    map[int]Call
}

type Notify struct {
}

type Call struct {
}

type Method struct {
}

func (n *Notify) call(t Transporter) error {
	return nil
}

func (d *Dispatch) lookupNotify(m string) *Notify {
	return nil
}

func (d *Dispatch) DispatchTriple(t Transporter) error {
	var l int
	var e Errors
	if e.Push(t.Decode(&l)) && l != TYPE_NOTIFY {
		e.Push(NewDispatcherError("expected NOTIFY=%d; got %d", TYPE_NOTIFY, l))
	}

	var m string
	if !e.Push(t.Decode(&m)) {
		var dummy interface{}
		e.Push(t.Decode(&dummy))
	} else if ntfy := d.lookupNotify(m); ntfy != nil {
		ntfy.call(t)
	} else {
		e.Push(NewDispatcherError("unknown notify: %s", m))
	}

	return e.Error()
}
