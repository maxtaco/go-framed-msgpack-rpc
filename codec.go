package rpc

import (
	"bytes"
	"github.com/ugorji/go/codec"
	"io/ioutil"
)

type Decoder interface {
	Decode(interface{}) error
}

type Encoder interface {
	Encode(interface{}) error
}

type ByteEncoder interface {
	EncodeToBytes(interface{}) ([]byte, error)
}

type FramedMsgpackEncoder struct {
	handle codec.Handle
}

func NewFramedMsgpackEncoder(mh *codec.MsgpackHandle) *FramedMsgpackEncoder {
	if mh == nil {
		mh = &codec.MsgpackHandle{WriteExt: true}
	}
	return &FramedMsgpackEncoder{
		handle: mh,
	}
}

func (e *FramedMsgpackEncoder) encodeToBytes(i interface{}) (v []byte, err error) {
	buf := new(bytes.Buffer)
	enc := codec.NewEncoder(buf, e.handle)
	if err = enc.Encode(i); err != nil {
		return
	}
	v, _ = ioutil.ReadAll(buf)
	return
}

func (e *FramedMsgpackEncoder) EncodeToBytes(i interface{}) (bytes []byte, err error) {
	var length, content []byte
	if content, err = e.encodeToBytes(i); err != nil {
		return
	}
	l := len(content)
	if length, err = e.encodeToBytes(l); err != nil {
		return
	}
	bytes = append(length, content...)
	return bytes, nil
}
