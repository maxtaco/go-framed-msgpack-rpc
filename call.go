package rpc

import (
	"io"
	"sync"

	"golang.org/x/net/context"
)

type call struct {
	ctx context.Context

	// resultCh serializes the possible results generated for this
	// call, with the first one becoming the true result.
	resultCh chan error

	// doneCh is closed when the true result for this call is
	// chosen.
	doneCh chan struct{}

	method         string
	seqid          seqNumber
	arg            interface{}
	res            interface{}
	profiler       Profiler
	errorUnwrapper ErrorUnwrapper
}

func newCall(ctx context.Context, m string, arg interface{}, res interface{}, u ErrorUnwrapper, p Profiler) *call {
	return &call{
		ctx:            ctx,
		resultCh:       make(chan error),
		doneCh:         make(chan struct{}),
		method:         m,
		arg:            arg,
		res:            res,
		profiler:       p,
		errorUnwrapper: u,
	}
}

// Finish tries to set the given error as the result of this call, and
// returns whether or not this was successful.
func (c *call) Finish(err error) bool {
	select {
	case c.resultCh <- err:
		close(c.doneCh)
		return true
	case <-c.doneCh:
		return false
	}
}

type notify struct {
	name     string
	arg      interface{}
	resultCh chan error
}

type callRetrieval struct {
	seqid seqNumber
	ch    chan *call
}

type callContainer struct {
	calls  map[seqNumber]*call
	callCh chan *callRetrieval
	mtx    sync.Mutex
}

func newCallContainer() *callContainer {
	return &callContainer{
		calls:  make(map[seqNumber]*call),
		callCh: make(chan *callRetrieval),
	}
}

func (cc *callContainer) addCall(c *call) {
	cc.mtx.Lock()
	defer cc.mtx.Unlock()

	cc.calls[c.seqid] = c
}

func (cc *callContainer) retrieveCall(seqid seqNumber) *call {
	cc.mtx.Lock()
	defer cc.mtx.Unlock()

	call := cc.calls[seqid]
	delete(cc.calls, seqid)
	return call
}

func (cc *callContainer) cleanupCalls() {
	cc.mtx.Lock()
	defer cc.mtx.Unlock()

	for _, c := range cc.calls {
		_ = c.Finish(io.EOF)
	}
}
