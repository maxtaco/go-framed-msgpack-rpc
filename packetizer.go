package rpc

import (
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/ugorji/go/codec"
)

type packetizer interface {
	NextFrame() (RPCData, error)
}

type packetHandler struct {
	dec          decoder
	reader       io.Reader
	frameDecoder *codec.Decoder
	protocols    *protocolHandler
	calls        *callContainer
}

func newPacketHandler(reader io.Reader, protocols *protocolHandler, calls *callContainer) *packetHandler {
	return &packetHandler{
		reader:       reader,
		dec:          codec.NewDecoder(reader, newCodecMsgpackHandle()),
		frameDecoder: codec.NewDecoderBytes([]byte{}, newCodecMsgpackHandle()),
		protocols:    protocols,
		calls:        calls,
	}
}

func (p *packetHandler) NextFrame() (RPCData, error) {
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
	p.frameDecoder.ResetBytes(bytes[1:])

	return DecodeRPC(nb-0x90, p.frameDecoder, p.protocols, p.calls)
}

func (p *packetHandler) loadNextFrame() ([]byte, error) {
	// Get the packet length
	var l int
	if err := p.dec.Decode(&l); err != nil {
		if _, ok := err.(*net.OpError); ok {
			// If the connection is reset or has been closed on this side,
			// return EOF
			return nil, io.EOF
		}
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
