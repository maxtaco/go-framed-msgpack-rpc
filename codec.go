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

type writeContainer struct {
	bytes []byte
	ch    chan error
}

type framedMsgpackEncoder struct {
	encoders encoderPool
	writer   io.Writer
	writeCh  chan writeContainer
	doneCh   chan struct{}
	closedCh chan struct{}
}

func newFramedMsgpackEncoder(writer io.Writer) *framedMsgpackEncoder {
	e := &framedMsgpackEncoder{
		encoders: makeEncoderPool(),
		writer:   writer,
		writeCh:  make(chan writeContainer),
		doneCh:   make(chan struct{}),
		closedCh: make(chan struct{}),
	}
	go e.writerLoop()
	return e
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
	write := writeContainer{bytes, make(chan error)}
	select {
	case <-e.doneCh:
		return io.EOF
	case e.writeCh <- write:
		return <-write.ch
	}
}

func (e *framedMsgpackEncoder) writerLoop() {
	for {
		select {
		case <-e.doneCh:
			close(e.closedCh)
			return
		case write := <-e.writeCh:
			_, err := e.writer.Write(write.bytes)
			write.ch <- err
		}
	}
}

func (e *framedMsgpackEncoder) Close() <-chan struct{} {
	close(e.doneCh)
	return e.closedCh
}
