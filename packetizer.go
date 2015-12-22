package rpc

import (
	"bufio"
	"errors"
	"fmt"
	"sync"

	"github.com/ugorji/go/codec"
)

const decoderPoolLength int = 10

type decoderPool chan *codec.Decoder

func makeDecoderPool() decoderPool {
	p := make(decoderPool, decoderPoolLength)

	for i := 0; i < decoderPoolLength; i++ {
		p <- codec.NewDecoderBytes([]byte{}, newCodecMsgpackHandle())
	}

	return p
}

type packetizer interface {
	NextFrame(*protocolHandler) (*RPCCall, error)
}

type packetHandler struct {
	dec           *codec.Decoder
	reader        *bufio.Reader
	frameDecoders decoderPool
	mtx           sync.Mutex
}

func newPacketHandler(reader *bufio.Reader) *packetHandler {
	return &packetHandler{
		reader:        reader,
		dec:           codec.NewDecoder(reader, newCodecMsgpackHandle()),
		frameDecoders: makeDecoderPool(),
	}
}

func (p *packetHandler) getFrameDecoder(bytes []byte) *codec.Decoder {
	dec := <-p.frameDecoders
	dec.ResetBytes(bytes)
	return dec
}

func (p *packetHandler) returnFrameDecoder(d *codec.Decoder) {
	p.frameDecoders <- d
}

func (p *packetHandler) NextFrame(protHandler *protocolHandler) (*RPCCall, error) {
	bytes, err := p.loadNextFrame()
	if err != nil {
		return nil, err
	}
	if len(bytes) < 1 {
		return nil, fmt.Errorf("invalid frame size: %d", len(bytes))
	}

	dec := p.getFrameDecoder(bytes[1:])
	defer p.returnFrameDecoder(dec)

	// Attempt to read the fixarray
	nb := int(bytes[0])

	// Interpret the byte as the length field of a fixarray of up
	// to 15 elements: see
	// https://github.com/msgpack/msgpack/blob/master/spec.md#formats-array
	// . Do this so we can decode directly into the expected
	// fields without copying.
	if nb < 0x91 || nb > 0x9f {
		return nil, NewPacketizerError("wrong message structure prefix (%d)", nb)
	}
	rpc := &RPCCall{}
	err = rpc.Decode(nb-1, dec, protHandler)
	if err != nil {
		return nil, err
	}
	return rpc, nil
}

func (p *packetHandler) loadNextFrame() ([]byte, error) {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	// Get the packet length
	var l int
	err := p.dec.Decode(&l)
	if err != nil {
		return nil, err
	}

	bytes, err := p.reader.Peek(l)
	if err != nil {
		return nil, err
	}
	discarded, err := p.reader.Discard(l)
	if err != nil {
		return nil, err
	}
	if discarded != l {
		return nil, errors.New("discarded wrong number of bytes")
	}
	return bytes, nil
}
