package rpc

import (
	"io"

	"golang.org/x/net/context"
)

type dispatcher interface {
	Call(ctx context.Context, name string, arg interface{}, res interface{}, u ErrorUnwrapper) error
	Notify(ctx context.Context, name string, arg interface{}) error
	Close(err error) chan struct{}
}

type dispatch struct {
	writer encoder
	calls  *callContainer

	seqid seqNumber

	// Stops all loops when closed
	stopCh chan struct{}
	// Closed once all loops are finished
	closedCh chan struct{}

	callCh   chan *call
	notifyCh chan *notify

	log LogInterface
}

func newDispatch(enc encoder, calls *callContainer, l LogInterface) *dispatch {
	d := &dispatch{
		writer:   enc,
		calls:    calls,
		callCh:   make(chan *call),
		notifyCh: make(chan *notify),
		stopCh:   make(chan struct{}),
		closedCh: make(chan struct{}),

		seqid: 0,
		log:   l,
	}
	go d.callLoop()
	return d
}

func (d *dispatch) callLoop() {
	for {
		select {
		case <-d.stopCh:
			d.calls.cleanupCalls()
			close(d.closedCh)
			return
		case c := <-d.callCh:
			d.handleCall(c)
		case n := <-d.notifyCh:
			d.handleNotify(n)
		case cr := <-d.calls.callCh:
			cr.ch <- d.calls.retrieveCall(cr.seqid)
		}
	}
}

func (d *dispatch) handleCall(c *call) {
	seqid := d.nextSeqid()
	c.seqid = seqid
	d.calls.addCall(c)
	v := []interface{}{MethodCall, seqid, c.method, c.arg}
	err := d.writer.Encode(v)
	if err != nil {
		setResult := c.Finish(err)
		if !setResult {
			panic("c.Finish unexpectedly returned false")
		}
		return
	}
	d.log.ClientCall(seqid, c.method, c.arg)
	go func() {
		select {
		case <-c.ctx.Done():
			setResult := c.Finish(newCanceledError(c.method, seqid))
			if !setResult {
				// The result of the call has already
				// been processed.
				return
			}
			// TODO: Remove c from calls:
			// https://github.com/keybase/go-framed-msgpack-rpc/issues/30
			// .
			v := []interface{}{MethodCancel, seqid, c.method}
			err := d.writer.Encode(v)
			d.log.ClientCancel(seqid, c.method, err)
		case <-c.doneCh:
		}
	}()
}

func (d *dispatch) handleNotify(n *notify) {
	err := d.writer.Encode([]interface{}{MethodNotify, n.name, n.arg})
	if err != nil {
		n.resultCh <- err
	}
	d.log.ClientNotify(n.name, n.arg)
	n.resultCh <- nil
}

func (d *dispatch) nextSeqid() seqNumber {
	ret := d.seqid
	d.seqid++
	return ret
}

func (d *dispatch) Call(ctx context.Context, name string, arg interface{}, res interface{}, u ErrorUnwrapper) error {
	profiler := d.log.StartProfiler("call %s", name)
	call := newCall(ctx, name, arg, res, u, profiler)
	select {
	case d.callCh <- call:
		return <-call.resultCh
	case <-d.stopCh:
		return io.EOF
	}
}

func (d *dispatch) Notify(ctx context.Context, name string, arg interface{}) error {
	notify := &notify{
		name:     name,
		arg:      arg,
		resultCh: make(chan error),
	}
	select {
	case d.notifyCh <- notify:
		return <-notify.resultCh
	case <-d.stopCh:
		return io.EOF
	case <-ctx.Done():
		return newCanceledError(name, -1)
	}
}

func (d *dispatch) Close(err error) chan struct{} {
	close(d.stopCh)
	return d.closedCh
}

func wrapError(f WrapErrorFunc, e error) interface{} {
	if f != nil {
		return f(e)
	}
	if e == nil {
		return nil
	}
	return e.Error()
}
