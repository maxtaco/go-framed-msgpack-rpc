package rpc

import (
	"io"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func testReceive(t *testing.T, p *Protocol, rpc rpcMessage) error {
	conn1, conn2 := net.Pipe()
	receiveOut := newFramedMsgpackEncoder(conn1)

	protHandler := createMessageTestProtocol()
	if p != nil {
		protHandler.registerProtocol(*p)
	}
	pkt := newPacketHandler(conn2, protHandler, newCallContainer())
	logFactory := NewSimpleLogFactory(SimpleLogOutput{}, SimpleLogOptions{})

	r := newReceiveHandler(receiveOut, protHandler, logFactory.NewLog(nil))

	errCh := make(chan error)
	go func() {
		_, err := pkt.NextFrame()
		errCh <- err
	}()

	err := r.Receive(rpc)
	if err != nil {
		return err
	}

	return <-errCh
}

func makeCall(seq seqNumber, name string, arg interface{}) *rpcCallMessage {
	return &rpcCallMessage{
		seqno: seq,
		name:  name,
		arg:   arg,
	}
}

func makeResponse(err error, res interface{}) *rpcResponseMessage {
	return &rpcResponseMessage{
		err: err,
		c: &call{
			resultCh: make(chan *rpcResponseMessage),
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
	c := &rpcResponseMessage{c: &call{}}
	done := runInBg(func() error {
		err := testReceive(
			t,
			nil,
			c,
		)
		return err
	})

	err := <-done
	recoverableError, ok := err.(RecoverableError)
	require.True(t, ok)
	require.True(t, recoverableError.CanRecover())
	require.EqualError(t, err, "Call not found for sequence number 0")
}

func TestCloseReceiver(t *testing.T) {
	logFactory := NewSimpleLogFactory(SimpleLogOutput{}, SimpleLogOptions{})
	conn1, _ := net.Pipe()
	receiveOut := newFramedMsgpackEncoder(conn1)
	r := newReceiveHandler(
		receiveOut,
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
