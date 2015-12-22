package rpc

import (
	"bufio"
	"net"

	"github.com/ugorji/go/codec"
)

type WrapErrorFunc func(error) interface{}

type Transporter interface {
	getDispatcher() (dispatcher, error)
	getReceiver() (receiver, error)
	Run() error
	RunAsync() error
	IsConnected() bool
	AddCloseListener(ch chan<- error)
	RegisterProtocol(p Protocol) error
}

type transporter interface {
	Transporter
}

type connDecoder struct {
	decoder
	net.Conn
	Reader *bufio.Reader
}

func newConnDecoder(c net.Conn) *connDecoder {
	br := bufio.NewReader(c)
	mh := &codec.MsgpackHandle{WriteExt: true}

	return &connDecoder{
		Conn:    c,
		Reader:  br,
		decoder: codec.NewDecoder(br, mh),
	}
}

var _ transporter = (*transport)(nil)

type transport struct {
	cdec            *connDecoder
	dispatcher      dispatcher
	receiver        receiver
	packetizer      packetizer
	protocolHandler *protocolHandler
	log             LogInterface
	wrapError       WrapErrorFunc
	startCh         chan struct{}
	stopCh          chan struct{}
}

func NewTransport(c net.Conn, l LogFactory, wef WrapErrorFunc) Transporter {
	cdec := newConnDecoder(c)
	if l == nil {
		l = NewSimpleLogFactory(nil, nil)
	}
	log := l.NewLog(cdec.RemoteAddr())

	startCh := make(chan struct{}, 1)
	startCh <- struct{}{}

	ret := &transport{
		cdec:            cdec,
		log:             log,
		wrapError:       wef,
		startCh:         startCh,
		stopCh:          make(chan struct{}),
		protocolHandler: newProtocolHandler(wef),
	}
	enc := newFramedMsgpackEncoder(ret.cdec)
	callRetrievalCh := make(chan callRetrieval)
	ret.dispatcher = newDispatch(enc, callRetrievalCh, log)
	ret.receiver = newReceiveHandler(enc, callRetrievalCh, log, wef)
	ret.packetizer = newPacketHandler(cdec.Reader)
	return ret
}

func (t *transport) IsConnected() bool {
	select {
	case <-t.stopCh:
		return false
	default:
		return true
	}
}

func (t *transport) Run() error {
	if !t.IsConnected() {
		return DisconnectedError{}
	}

	select {
	case <-t.startCh:
		return t.run()
	default:
		return nil
	}
}

func (t *transport) RunAsync() error {
	if !t.IsConnected() {
		return DisconnectedError{}
	}

	select {
	case <-t.startCh:
		go func() {
			err := t.run()
			if err != nil {
				t.log.Warning("asynchronous t.run() failed with %v", err)
			}
		}()
	default:
	}

	return nil
}

func (t *transport) run() (err error) {
	// Initialize transport loops
	writerDone := runInBg(t.writerLoop)

	// Packetize: do work
	for {
		var rpc *RPCCall
		if rpc, err = t.packetizer.NextFrame(t.protocolHandler); err != nil {
			t.receiver.Receive(rpc)
			continue
		}
		break
	}

	// Log packetizer completion
	t.log.TransportError(err)

	// Since the receiver might require the transport, we have to
	// close it before terminating our loops
	<-t.dispatcher.Close(err)
	<-t.receiver.Close(err)
	close(t.stopCh)

	// Wait for loops to finish before closing the connection
	<-writerDone

	// Cleanup
	t.cdec.Close()

	return
}

func (t *transport) getDispatcher() (dispatcher, error) {
	if !t.IsConnected() {
		return nil, DisconnectedError{}
	}
	return t.dispatcher, nil
}

func (t *transport) getReceiver() (receiver, error) {
	if !t.IsConnected() {
		return nil, DisconnectedError{}
	}
	return t.receiver, nil
}

func (t *transport) RegisterProtocol(p Protocol) error {
	return t.protHandler.registerProtocol(p)
}

func (t *transport) AddCloseListener(ch chan<- error) {
	return t.receiver.AddCloseListener(ch)
}
