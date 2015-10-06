package rpc

import (
	"github.com/ugorji/go/codec"
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

func NewFramedMsgpackEncoder() *FramedMsgpackEncoder {
	mh := &codec.MsgpackHandle{WriteExt: true}
	return &FramedMsgpackEncoder{
		handle: mh,
	}
}

func (e *FramedMsgpackEncoder) encodeToBytes(i interface{}) (v []byte, err error) {
	v = make([]byte, 0)
	enc := codec.NewEncoderBytes(&v, e.handle)
	if err = enc.Encode(i); err != nil {
		return
	}
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
