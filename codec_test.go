package rpc

import (
	"bytes"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ugorji/go/codec"
	"golang.org/x/net/context"
)

// This test determines the behavior of codec with respect to advancing the
// Reader when decoding error scenarios. It seems that the codec advances
// the Reader if Decode fails, but sets its own state to expect a specific
// type for the next Decode, and thus is functionally the same as not
// advancing the Reader.
func TestCodec(t *testing.T) {
	var buf bytes.Buffer
	mh := &codec.MsgpackHandle{WriteExt: true}
	enc := codec.NewEncoder(&buf, mh)
	dec := codec.NewDecoder(&buf, mh)

	var i int = math.MaxInt32
	err := enc.Encode(i)
	require.Nil(t, err, "expected encoding to succeed")
	require.Equal(t, 5, len(buf.Bytes()), "expected buffer to contain bytes")

	var targetInt int
	err = dec.Decode(&targetInt)
	require.Nil(t, err, "expected decoding to succeed")
	require.Equal(t, math.MaxInt32, targetInt, "expected codec to successfully decode int")
	require.Equal(t, 0, len(buf.Bytes()), "expected buffer to be empty")

	var targetString string
	enc.Encode(i)
	require.Equal(t, 5, len(buf.Bytes()), "expected buffer to contain bytes")
	err = dec.Decode(&targetString)
	require.Error(t, err, "expected error while decoding")
	require.Contains(t, err.Error(), "Unrecognized descriptor byte", "expected error while decoding")
	require.Equal(t, 4, len(buf.Bytes()), "expected buffer to have bytes")
	err = dec.Decode(&targetString)
	require.Error(t, err, "expected error while decoding")
	require.Contains(t, err.Error(), "Unrecognized descriptor byte", "expected error while decoding")
	require.Equal(t, 4, len(buf.Bytes()), "expected buffer to have bytes")

	targetInt = 0
	err = dec.Decode(&targetInt)
	require.Nil(t, err, "expected decoding to succeed")
	require.Equal(t, math.MaxInt32, targetInt, "expected codec to successfully decode int")
	require.Equal(t, 0, len(buf.Bytes()), "expected buffer to be empty")
}

func TestCodecStruct(t *testing.T) {
	var buf bytes.Buffer
	mh := &codec.MsgpackHandle{
		WriteExt: true,
	}
	enc := codec.NewEncoder(&buf, mh)
	dec := codec.NewDecoder(&buf, mh)

	v := []interface{}{MethodCall, 999, "abc.hello", new(interface{})}

	err := enc.Encode(v)
	require.Nil(t, err, "expected encoding to succeed")
	require.Equal(t, 16, len(buf.Bytes()), "expected buffer to contain bytes")

	// Advance the buffer past the msgpack fixarray descriptor byte
	b, _ := buf.ReadByte()
	require.Equal(t, 0x94, int(b))

	p := newProtocolHandler(nil)
	p.registerProtocol(Protocol{
		Name: "abc",
		Methods: map[string]ServeHandlerDescription{
			"hello": {
				MakeArg: func() interface{} {
					return nil
				},
				Handler: func(context.Context, interface{}) (interface{}, error) {
					return nil, nil
				},
				MethodType: MethodCall,
			},
		},
	})
	c := RPCCall{}
	err = c.Decode(4, dec, p, nil)
	require.Nil(t, err)
	require.Equal(t, MethodCall, c.Type())
	require.Equal(t, seqNumber(999), c.SeqNo())
	require.Equal(t, "abc.hello", c.Name())
	require.Equal(t, nil, c.Arg())
}
