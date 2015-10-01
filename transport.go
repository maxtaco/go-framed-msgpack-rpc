package rpc

import (
	"bufio"
	"github.com/ugorji/go/codec"
	"io"
	"net"
	"sync"
)

type WrapErrorFunc func(error) interface{}
type UnwrapErrorFunc func(nxt DecodeNext) (error, error)

type Transporter interface {
	getDispatcher() (dispatcher, error)
	Run(bool) error
	IsConnected() bool
}

type transporter interface {
	Transporter
	io.ByteReader
	Decoder
	Encoder
	sync.Locker
}

type connDecoder struct {
	Decoder
	net.Conn
	io.ByteReader
}

func newConnDecoder(c net.Conn, mh *codec.MsgpackHandle) *connDecoder {
	br := bufio.NewReader(c)

	return &connDecoder{
		Conn:       c,
		ByteReader: br,
		Decoder:    codec.NewDecoder(br, mh),
	}
}

type Transport struct {
	sync.Locker
	ByteEncoder
	cdec           *connDecoder
	dispatcher     dispatcher
	packetizer     *packetizer
	log            LogInterface
	wrapError      WrapErrorFunc
	writeCh        chan []byte
	readByteCh     chan struct{}
	decodeCh       chan interface{}
	readerResultCh chan interface{}
	writerResultCh chan error
	startedCh      chan struct{}
	stopCh         chan struct{}
}

func NewTransport(c net.Conn, l LogFactory, wef WrapErrorFunc) *Transport {
	mh := &codec.MsgpackHandle{WriteExt: true}
	cdec := newConnDecoder(c, mh)
	if l == nil {
		l = NewSimpleLogFactory(nil, nil)
	}
	log := l.NewLog(cdec.RemoteAddr())
	byteEncoder := NewFramedMsgpackEncoder(mh)

	ret := &Transport{
		Locker:         new(sync.Mutex),
		ByteEncoder:    byteEncoder,
		cdec:           cdec,
		log:            log,
		wrapError:      wef,
		writeCh:        make(chan []byte),
		readByteCh:     make(chan struct{}),
		decodeCh:       make(chan interface{}),
		readerResultCh: make(chan interface{}),
		writerResultCh: make(chan error),
		startedCh:      make(chan struct{}, 1),
		stopCh:         make(chan struct{}),
	}
	ret.dispatcher = NewDispatch(ret, log, wef)
	ret.packetizer = NewPacketizer(ret.dispatcher, ret)
	// Make one token available to start
	select {
	case ret.startedCh <- struct{}{}:
	default:
	}
	return ret
}

func (t *Transport) IsConnected() bool {
	select {
	case <-t.stopCh:
		return false
	default:
		return true
	}
}

func (t *Transport) handlePacketizerFailure(err error) {
	// For now, just throw everything away.  Eventually we might
	// want to make a plan for reconnecting.
	// TODO JZ: Not sure what to do with this comment & log
	// NOTE: The logging implementation can be anything. In particular, it
	// might try to send logs over this transport, which would take the mutex
	// again. We *must not* call this while we hold the lock. (Yes, we figured
	// this out by deadlocking ourselves :p)
	t.log.TransportError(err)
	close(t.stopCh)
	return
}

func (t *Transport) Run(bg bool) (err error) {
	if !t.IsConnected() {
		return DisconnectedError{}
	}
	select {
	case <-t.startedCh:
		if bg {
			go t.run2()
		} else {
			return t.run2()
		}
	default:
	}
	return
}

func (t *Transport) run2() (err error) {
	readerDone := t.readerLoop()
	writerDone := t.writerLoop()
	err = t.packetizer.Packetize()
	t.handlePacketizerFailure(err)
	<-readerDone
	<-writerDone
	t.reset()
	return
}

func (t *Transport) readerLoop() chan struct{} {
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-t.stopCh:
				return
			case i := <-t.decodeCh:
				err := t.cdec.Decode(i)
				t.readerResultCh <- err
			case <-t.readByteCh:
				b, err := t.cdec.ReadByte()
				res := byteResult{
					b:   b,
					err: err,
				}
				t.readerResultCh <- res
			}
		}
		close(done)
	}()
	return done
}

func (t *Transport) writerLoop() chan struct{} {
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-t.stopCh:
				return
			case bytes := <-t.writeCh:
				_, err := t.cdec.Write(bytes)
				t.writerResultCh <- err
			}
		}
		close(done)
	}()
	return done
}

func (t *Transport) reset() {
	t.dispatcher.Reset()
	t.dispatcher = nil
	t.packetizer.Clear()
	t.packetizer = nil
	t.cdec.Close()
	t.cdec = nil
}

type byteResult struct {
	b   byte
	err error
}

func (t *Transport) Encode(i interface{}) error {
	bytes, err := t.EncodeToBytes(i)
	if err != nil {
		return err
	}
	t.writeCh <- bytes
	err = <-t.writerResultCh
	return err
}

func (t *Transport) ReadByte() (byte, error) {
	t.readByteCh <- struct{}{}
	res := <-t.readerResultCh
	byteRes, _ := res.(byteResult)
	return byteRes.b, byteRes.err
}

func (t *Transport) Decode(i interface{}) error {
	t.decodeCh <- i
	res := <-t.readerResultCh
	err, _ := res.(error)
	return err
}

func (t *Transport) getDispatcher() (dispatcher, error) {
	if !t.IsConnected() {
		return nil, DisconnectedError{}
	} else {
		return t.dispatcher, nil
	}
}
