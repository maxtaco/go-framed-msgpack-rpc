package rpc

import (
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/ugorji/go/codec"
)

type packetizer interface {
	NextFrame() (RPCMessage, error)
}

type packetHandler struct {
	dec           decoder
	reader        io.Reader
	frameDecoders decoderPool
	protocols     *protocolHandler
	calls         *callContainer
	mtx           sync.Mutex
}

func newPacketHandler(reader io.Reader, protocols *protocolHandler, calls *callContainer) *packetHandler {
	return &packetHandler{
		reader:        reader,
		dec:           codec.NewDecoder(reader, newCodecMsgpackHandle()),
		frameDecoders: makeDecoderPool(),
		protocols:     protocols,
		calls:         calls,
	}
}

func (p *packetHandler) NextFrame() (RPCMessage, error) {
	bytes, err := p.loadNextFrame()
	if err != nil {
		return nil, err
	}
	if len(bytes) < 1 {
		return nil, fmt.Errorf("invalid frame size: %d", len(bytes))
	}

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
	dec := p.frameDecoders.getDecoder(bytes[1:])
	defer p.frameDecoders.returnDecoder(dec)

	rpc := &RPCCall{}
	err = rpc.Decode(nb-0x90, dec, p.protocols, p.calls)
	return rpc, err
}

func (p *packetHandler) loadNextFrame() ([]byte, error) {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	// Get the packet length
	var l int
	err := p.dec.Decode(&l)
	if err != nil {
		fmt.Printf("read length failed with error: %+v\n", err)
		return nil, err
	}

	bytes := make([]byte, l)
	len, err := p.reader.Read(bytes)
	if err != nil {
		return nil, err
	}
	if len != l {
		return nil, errors.New("Unable to read desired length")
	}
	return bytes, nil
}
