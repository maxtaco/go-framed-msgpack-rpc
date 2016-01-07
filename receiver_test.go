package rpc

import (
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func testReceive(t *testing.T, p *Protocol, rpc RPCData) error {
	receiveOut := newBlockingMockCodec()

	logFactory := NewSimpleLogFactory(SimpleLogOutput{}, SimpleLogOptions{})
	protHandler := newProtocolHandler(nil)
	r := newReceiveHandler(receiveOut, protHandler, logFactory.NewLog(nil))
	if p != nil {
		protHandler.registerProtocol(*p)
	}

	errCh := make(chan error)
	go func() {
		var m MethodType
		var seqno seqNumber
		var respErrStr string
		var res interface{}
		decErr := receiveOut.Decode(&m)
		require.Nil(t, decErr, "Expected decode to succeed")
		require.Equal(t, MethodResponse, m, "Expected a response")
		decErr = receiveOut.Decode(&seqno)
		require.Nil(t, decErr, "Expected decode to succeed")
		require.Equal(t, seqNumber(0), seqno, "Expected sequence number to be 0")
		decErr = receiveOut.Decode(&respErrStr)
		require.Nil(t, decErr, "Expected decode to succeed")
		decErr = receiveOut.Decode(&res)
		require.Nil(t, decErr, "Expected decode to succeed")
		errCh <- errors.New(respErrStr)
	}()

	err := r.Receive(rpc)
	if err != nil {
		return err
	}

	return <-errCh
}

func makeCall(seq seqNumber, name string, arg interface{}) *RPCCallData {
	return &RPCCallData{
		seqno: seq,
		name:  name,
		arg:   arg,
	}
}

func makeResponse(err error, res interface{}) *RPCResponseData {
	return &RPCResponseData{
		err: err,
		c: &call{
			resultCh: make(chan *RPCResponseData),
			res:      res,
		},
	}
}

func TestReceiveResponse(t *testing.T) {
	c := makeResponse(
		nil,
		"hi",
	)
	go func() {
		err := testReceive(
			t,
			nil,
			c,
		)
		require.Nil(t, err)
	}()

	resp := <-c.ResponseCh()
	require.Equal(t, "hi", resp.Res())
}

func TestReceiveResponseNilCall(t *testing.T) {
	c := &RPCResponseData{c: &call{}}
	done := runInBg(func() error {
		err := testReceive(
			t,
			nil,
			c,
		)
		return err
	})

	err := <-done
	require.EqualError(t, err, "Call not found for sequence number 0")
}

func TestCloseReceiver(t *testing.T) {
	logFactory := NewSimpleLogFactory(SimpleLogOutput{}, SimpleLogOptions{})
	r := newReceiveHandler(
		newBlockingMockCodec(),
		newProtocolHandler(nil),
		logFactory.NewLog(nil),
	)
	// Buffer so the write doesn't block and thus we don't need a goroutine
	errCh := make(chan error, 1)

	r.AddCloseListener(errCh)
	r.Close(io.EOF)

	err := <-errCh
	require.EqualError(t, err, io.EOF.Error())
}
