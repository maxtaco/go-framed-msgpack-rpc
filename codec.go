package rpc

import (
	"io"

	"github.com/ugorji/go/codec"
)

type decoder interface {
	Decode(interface{}) error
}

type encoder interface {
	Encode(interface{}) error
}

const poolSize int = 10

type decoderPool chan *codec.Decoder

func makeDecoderPool() decoderPool {
	p := make(decoderPool, poolSize)

	for i := 0; i < poolSize; i++ {
		p <- codec.NewDecoderBytes([]byte{}, newCodecMsgpackHandle())
	}

	return p
}

func (p decoderPool) getDecoder(bytes []byte) *codec.Decoder {
	dec := <-p
	dec.ResetBytes(bytes)
	return dec
}

func (p decoderPool) returnDecoder(dec *codec.Decoder) {
	p <- dec
}

type encoderPool chan *codec.Encoder

func makeEncoderPool() encoderPool {
	p := make(encoderPool, poolSize)

	for i := 0; i < poolSize; i++ {
		p <- codec.NewEncoderBytes(&[]byte{}, newCodecMsgpackHandle())
	}

	return p
}

func (p encoderPool) getEncoder(bytes *[]byte) *codec.Encoder {
	enc := <-p
	enc.ResetBytes(bytes)
	return enc
}

func (p encoderPool) returnEncoder(enc *codec.Encoder) {
	p <- enc
}

func newCodecMsgpackHandle() codec.Handle {
	return &codec.MsgpackHandle{
		WriteExt: true,
	}
}

type framedMsgpackEncoder struct {
	encoders encoderPool
	writer   io.Writer
}

func newFramedMsgpackEncoder(writer io.Writer) *framedMsgpackEncoder {
	return &framedMsgpackEncoder{
		encoders: makeEncoderPool(),
		writer:   writer,
	}
}

func (e *framedMsgpackEncoder) encodeToBytes(i interface{}) (v []byte, err error) {
	enc := e.encoders.getEncoder(&v)
	defer e.encoders.returnEncoder(enc)
	err = enc.Encode(i)
	return v, err
}

func (e *framedMsgpackEncoder) encodeFrame(i interface{}) ([]byte, error) {
	content, err := e.encodeToBytes(i)
	if err != nil {
		return nil, err
	}
	length, err := e.encodeToBytes(len(content))
	if err != nil {
		return nil, err
	}
	return append(length, content...), nil
}

func (e *framedMsgpackEncoder) Encode(i interface{}) error {
	bytes, err := e.encodeFrame(i)
	if err != nil {
		return err
	}
	_, err = e.writer.Write(bytes)
	return err
}
