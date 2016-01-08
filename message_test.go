package rpc

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
)

func createMessageTestProtocol() *protocolHandler {
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
	return p
}

func runMessageTest(t *testing.T, v []interface{}) (rpcMessage, error) {
	var buf bytes.Buffer
	enc := newFramedMsgpackEncoder(&buf)
	pkt := newPacketHandler(&buf, createMessageTestProtocol(), newCallContainer())

	err := <-enc.Encode(v)
	require.Nil(t, err, "expected encoding to succeed")

	return pkt.NextFrame()
}

func TestMessageDecodeValid(t *testing.T) {
	v := []interface{}{MethodCall, 999, "abc.hello", new(interface{})}

	rpc, err := runMessageTest(t, v)
	c, ok := rpc.(*rpcCallMessage)
	require.True(t, ok)
	require.Nil(t, err)
	require.Equal(t, MethodCall, c.Type())
	require.Equal(t, seqNumber(999), c.SeqNo())
	require.Equal(t, "abc.hello", c.Name())
	require.Equal(t, nil, c.Arg())
}

func TestMessageDecodeValidExtraParams(t *testing.T) {
	v := []interface{}{MethodCall, 999, "abc.hello", new(interface{}), "foo", "bar"}

	rpc, err := runMessageTest(t, v)
	c, ok := rpc.(*rpcCallMessage)
	require.True(t, ok)
	require.Nil(t, err)
	require.Equal(t, MethodCall, c.Type())
	require.Equal(t, seqNumber(999), c.SeqNo())
	require.Equal(t, "abc.hello", c.Name())
	require.Equal(t, nil, c.Arg())
}

func TestMessageDecodeInvalidType(t *testing.T) {
	v := []interface{}{"hello", seqNumber(0), "invalid", new(interface{})}

	_, err := runMessageTest(t, v)
	require.EqualError(t, err, "[pos 1]: Unhandled single-byte unsigned integer value: Unrecognized descriptor byte: a5")
}

func TestMessageDecodeInvalidMethodType(t *testing.T) {
	v := []interface{}{MethodType(999), seqNumber(0), "invalid", new(interface{})}

	_, err := runMessageTest(t, v)
	require.EqualError(t, err, "invalid RPC type")
}

func TestMessageDecodeInvalidProtocol(t *testing.T) {
	v := []interface{}{MethodCall, seqNumber(0), "nonexistent.broken", new(interface{})}

	_, err := runMessageTest(t, v)
	require.EqualError(t, err, "protocol not found: nonexistent")
}

func TestMessageDecodeInvalidMethod(t *testing.T) {
	v := []interface{}{MethodCall, seqNumber(0), "abc.invalid", new(interface{})}

	_, err := runMessageTest(t, v)
	require.EqualError(t, err, "method 'invalid' not found in protocol 'abc'")
}

func TestMessageDecodeWrongMessageLength(t *testing.T) {
	v := []interface{}{MethodCall, seqNumber(0), "abc.invalid"}

	_, err := runMessageTest(t, v)
	require.EqualError(t, err, "wrong message length")
}

func TestMessageDecodeResponseNilCall(t *testing.T) {
	v := []interface{}{MethodResponse, seqNumber(0), 32, "hi"}

	_, err := runMessageTest(t, v)
	require.EqualError(t, err, "Call not found for sequence number 0")
}
