package rpc2

type Dispatcher interface {
	Dispatch(msg Message) error
	DispatchTriple(t Transporter) error
	DispatchQuad(t Transporter) error
}

type Dispatch struct {
}

func (d *Dispatch) DispatchTriple(t Transporter) (err error) {
	var l int
	if err = t.Decode(&l); err != nil {
		return
	}
	if l != TYPE_NOTIFY {
		err = NewDispatcherError("expected NOTIFY=%d; got %d", TYPE_NOTIFY, l)
	}

	return
}
