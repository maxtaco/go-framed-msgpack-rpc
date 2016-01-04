package rpc

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func testReceive(t *testing.T, p *Protocol, rpc RPCMessage) error {
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

func makeCall(typ MethodType, seq seqNumber, name string, arg interface{}) RPCMessage {
	return &RPCCall{
		typ: typ,
		RPCData: &RPCCallData{
			seqno: seq,
			name:  name,
			arg:   arg,
		},
	}
}

//func TestReceiveResponse(t *testing.T) {
//	c := makeCall(
//		MethodResponse,
//		seqNumber(0),
//		"",
//		"hi",
//	)
//	go func() {
//		err := testReceive(
//			t,
//			nil,
//			c
//		)
//		require.Nil(t, err, "expected receive to succeed")
//	}()
//
//	resp := <-c.resultCh
//	require.Equal(t, "hi", resp.Res(), "Expected response to say \"hi\"")
//}
//
//func TestReceiveResponseWrongSize(t *testing.T) {
//	err := testReceive(
//		t,
//		nil,
//		makeCall(
//			MethodResponse,
//			seqNumber(0),
//			"",
//			nil,
//		),
//	)
//	require.EqualError(t, err, "dispatcher error: wrong number of fields for message (got n=3, expected n=4)", "Expected error attempting to receive the wrong size of response")
//}
//
//func TestReceiveResponseNilCall(t *testing.T) {
//	callCh := make(chan callRetrieval)
//	done := runInBg(func() error {
//		err := testReceive(
//			t,
//			nil,
//			makeCall(
//				MethodResponse,
//				seqNumber(0),
//				"",
//				"hi",
//			),
//		)
//		return err
//	})
//
//	callRetrieval := <-callCh
//	callRetrieval.ch <- nil
//
//	err := <-done
//	require.EqualError(t, err, "Call not found for sequence number 0", "expected error when passing in a nil call")
//}
//
//func TestReceiveResponseError(t *testing.T) {
//	callCh := make(chan callRetrieval)
//	done := runInBg(func() error {
//		err := testReceive(
//			t,
//			nil,
//			makeCall(
//				MethodResponse,
//				seqNumber(0),
//				// Wrong type for error, should be string
//				32,
//				"hi",
//			),
//		)
//		return err
//	})
//
//	var result string
//	c := newCall(context.Background(), "testmethod", new(interface{}), &result, nil, NilProfiler{})
//	callRetrieval := <-callCh
//	callRetrieval.ch <- c
//
//	err := <-c.resultCh
//	require.EqualError(t, err, "Tried to decode incorrect type. Expected: string, actual: int", "expected error when passing in a nil call")
//	err = <-done
//	require.EqualError(t, err, "Tried to decode incorrect type. Expected: string, actual: int", "expected error when passing in a nil call")
//}
//
//func TestCloseReceiver(t *testing.T) {
//	logFactory := NewSimpleLogFactory(SimpleLogOutput{}, SimpleLogOptions{})
//	r := newReceiveHandler(
//		newBlockingMockCodec(),
//		newBlockingMockCodec(),
//		make(chan callRetrieval),
//		logFactory.NewLog(nil),
//		nil,
//	)
//	// Buffer so the write doesn't block and thus we don't need a goroutine
//	errCh := make(chan error, 1)
//
//	r.AddCloseListener(errCh)
//	r.Close(io.EOF)
//
//	err := <-errCh
//	require.EqualError(t, err, io.EOF.Error(), "expected EOF error from closing the receiver")
//}
