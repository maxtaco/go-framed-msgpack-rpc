package rpc2

import (
	"bufio"
	"bytes"
	"errors"
	"github.com/ugorji/go/codec"
	"io/ioutil"
	"log"
	"net"
	"sync"
)

type WrapErrorFunc func(error) interface{}
type UnwrapErrorFunc func(nxt DecodeNext) (error, error)

type Transporter interface {
	RawWrite([]byte) error
	ReadByte() (byte, error)
	Decode(interface{}) error
	WrapError(error) interface{}
	Encode(interface{}) error
	UnwrapError(DecodeNext) (error, error)
	GetDispatcher() (Dispatcher, error)
	ReadLock()
	ReadUnlock()
}

type TransportHooks struct {
	wrapError   WrapErrorFunc
	unwrapError UnwrapErrorFunc
	warn        WarnFunc
}

type ConPackage struct {
	con net.Conn
	br  *bufio.Reader
	dec *codec.Decoder
}

func (c *ConPackage) ReadByte() (b byte, e error) {
	return c.br.ReadByte()
}

func (c *ConPackage) Write(b []byte) (err error) {
	_, err = c.con.Write(b)
	return
}

func (c *ConPackage) Close() error {
	return c.con.Close()
}

type Transport struct {
	hooks      *TransportHooks
	mh         *codec.MsgpackHandle
	cpkg       *ConPackage
	buf        *bytes.Buffer
	enc        *codec.Encoder
	mutex      *sync.Mutex
	rdlck      *sync.Mutex
	dispatcher Dispatcher
	packetizer *Packetizer
	running    bool
}

func NewConPackage(c net.Conn, mh *codec.MsgpackHandle) *ConPackage {
	br := bufio.NewReader(c)
	return &ConPackage{
		con: c,
		br:  br,
		dec: codec.NewDecoder(br, mh),
	}
}

func (t *Transport) IsConnected() bool {
	t.mutex.Lock()
	ret := (t.cpkg != nil)
	t.mutex.Unlock()
	return ret
}

func NewTransport(c net.Conn, h *TransportHooks) *Transport {
	var mh codec.MsgpackHandle
	buf := new(bytes.Buffer)
	ret := &Transport{
		hooks: h,
		mh:    &mh,
		cpkg:  NewConPackage(c, &mh),
		buf:   buf,
		enc:   codec.NewEncoder(buf, &mh),
		mutex: new(sync.Mutex),
		rdlck: new(sync.Mutex),
	}
	ret.dispatcher = NewDispatch(ret, ret.getWarnFunc())
	ret.packetizer = NewPacketizer(ret.dispatcher, ret)
	return ret
}

func (t *Transport) ReadLock()   { t.rdlck.Lock() }
func (t *Transport) ReadUnlock() { t.rdlck.Unlock() }

func (t *Transport) encodeToBytes(i interface{}) (v []byte, err error) {
	if err = t.enc.Encode(i); err != nil {
		return
	}
	v, _ = ioutil.ReadAll(t.buf)
	return
}

func (t *Transport) getWarnFunc() (ret WarnFunc) {
	if t.hooks != nil {
		ret = t.hooks.warn
	}
	if ret == nil {
		ret = func(s string) {
			log.Print(s)
		}
	}
	return
}

func (t *Transport) warn(s string) {
	t.getWarnFunc()(s)
}

func (t *Transport) run2() (err error) {
	err = t.packetizer.Packetize()
	t.handlePacketizerFailure(err)
	return
}

func (t *Transport) handlePacketizerFailure(err error) {
	// For now, just throw everything away.  Eventually we might
	// want to make a plan for reconnecting.
	t.mutex.Lock()
	t.warn(err.Error())
	t.running = false
	t.dispatcher.Reset()
	t.dispatcher = nil
	t.packetizer.Clear()
	t.packetizer = nil
	t.cpkg.Close()
	t.cpkg = nil
	t.mutex.Unlock()
	return
}

func (t *Transport) run(bg bool) (err error) {
	dostart := false
	t.mutex.Lock()
	if t.cpkg == nil {
		err = DisconnectedError{}
	} else if !t.running {
		dostart = true
		t.running = true
	}
	t.mutex.Unlock()
	if dostart {
		if bg {
			go t.run2()
		} else {
			err = t.run2()
		}
	}
	return
}

func (t *Transport) Encode(i interface{}) (err error) {
	var v1, v2 []byte
	if v2, err = t.encodeToBytes(i); err != nil {
		return
	}
	l := len(v2)
	if v1, err = t.encodeToBytes(l); err != nil {
		return
	}
	if err = t.RawWrite(v1); err != nil {
		return
	}
	return t.RawWrite(v2)
}

func (t *Transport) WrapError(e error) interface{} {
	if t.hooks != nil && t.hooks.wrapError != nil {
		return t.hooks.wrapError(e)
	} else if e == nil {
		return nil
	} else {
		return e.Error()
	}
}

func (t *Transport) getConPackage() (ret *ConPackage, err error) {
	t.mutex.Lock()
	ret = t.cpkg
	t.mutex.Unlock()
	if ret == nil {
		err = DisconnectedError{}
	}
	return
}

func (t *Transport) ReadByte() (b byte, err error) {
	var cp *ConPackage
	if cp, err = t.getConPackage(); err == nil {
		b, err = cp.ReadByte()
	}
	return
}

func (t *Transport) Decode(i interface{}) (err error) {
	var cp *ConPackage
	if cp, err = t.getConPackage(); err == nil {
		err = cp.dec.Decode(i)
	}
	return
}

func (t *Transport) UnwrapError(dnf DecodeNext) (app error, dispatch error) {
	var s string
	if t.hooks != nil && t.hooks.unwrapError != nil {
		app, dispatch = t.hooks.unwrapError(dnf)
	} else if dispatch = dnf(&s); dispatch == nil && len(s) > 0 {
		app = errors.New(s)
	} 
	return
}

func (t *Transport) RawWrite(b []byte) (err error) {
	var cp *ConPackage
	if cp, err = t.getConPackage(); err == nil {
		err = cp.Write(b)
	}
	return err
}

func (t *Transport) GetDispatcher() (d Dispatcher, err error) {
	t.run(true)
	if !t.IsConnected() {
		err = DisconnectedError{}
	} else {
		d = t.dispatcher
	}
	return
}
