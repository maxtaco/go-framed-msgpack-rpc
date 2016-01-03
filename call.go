package rpc

import (
	"sync"

	"golang.org/x/net/context"
)

type call struct {
	ctx context.Context

	// TODO respond with less than a full RPCMessage
	resultCh chan RPCMessage

	closedCh chan struct{}

	method         string
	seqid          seqNumber
	arg            interface{}
	res            interface{}
	errorUnwrapper ErrorUnwrapper
}

type callRetrieval struct {
	seqid seqNumber
	ch    chan *call
}

type callContainer struct {
	calls    map[seqNumber]*call
	callCh   chan *callRetrieval
	callsMtx sync.Mutex
	seqMtx   sync.Mutex
	seqid    seqNumber
}

func newCallContainer() *callContainer {
	return &callContainer{
		calls:  make(map[seqNumber]*call),
		callCh: make(chan *callRetrieval),
		seqid:  0,
	}
}

// TODO implement stopCh and closedCh scheme to handle closing the container

func (cc *callContainer) NewCall(ctx context.Context, m string, arg interface{}, res interface{}, u ErrorUnwrapper) *call {
	return &call{
		ctx:            ctx,
		resultCh:       make(chan RPCMessage),
		method:         m,
		arg:            arg,
		res:            res,
		errorUnwrapper: u,
		seqid:          cc.nextSeqid(),
	}
}

func (cc *callContainer) nextSeqid() seqNumber {
	cc.seqMtx.Lock()
	defer cc.seqMtx.Unlock()

	ret := cc.seqid
	cc.seqid++
	return ret
}

func (cc *callContainer) AddCall(c *call) {
	cc.callsMtx.Lock()
	defer cc.callsMtx.Unlock()

	cc.calls[c.seqid] = c
}

func (cc *callContainer) RetrieveCall(seqid seqNumber) *call {
	cc.callsMtx.Lock()
	defer cc.callsMtx.Unlock()

	return cc.calls[seqid]
}

func (cc *callContainer) RemoveCall(seqid seqNumber) {
	cc.callsMtx.Lock()
	defer cc.callsMtx.Unlock()

	delete(cc.calls, seqid)
}
