package rpc

import (
	"io"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
)

func decodeToNull(dec decoder, l int) error {
	for i := 0; i < l; i++ {
		if err := dec.Decode(new(interface{})); err != nil {
			return err
		}
	}
	return nil
}

func dispatchTestCallWithContext(t *testing.T, ctx context.Context) (dispatcher, *callContainer, chan error) {
	dispatchOut := newBlockingMockCodec()

	logFactory := NewSimpleLogFactory(SimpleLogOutput{}, SimpleLogOptions{})
	calls := newCallContainer()
	d := newDispatch(dispatchOut, calls, logFactory.NewLog(nil))

	done := runInBg(func() error {
		return d.Call(ctx, "whatever", new(interface{}), new(interface{}), nil)
	})

	// Necessary to ensure the call is far enough along to
	// be ready to respond
	decoderErr := decodeToNull(dispatchOut, 4)
	require.Nil(t, decoderErr, "Expected no error")
	return d, calls, done
}

func dispatchTestCall(t *testing.T) (dispatcher, *callContainer, chan error) {
	return dispatchTestCallWithContext(t, context.Background())
}

func TestDispatchSuccessfulCall(t *testing.T) {
	d, calls, done := dispatchTestCall(t)

	c := calls.retrieveCall(0)
	require.NotNil(t, c, "Expected c not to be nil")

	ok := c.Finish(nil)
	require.True(t, ok, "Expected c.Finish to succeed")
	err := <-done
	require.Nil(t, err, "Expected no error")

	closed := d.Close(nil)
	<-closed
}

func TestDispatchCanceledBeforeResult(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	d, calls, done := dispatchTestCallWithContext(t, ctx)

	c := calls.retrieveCall(0)
	require.NotNil(t, c, "Expected c not to be nil")

	cancel()

	err := <-done
	_, canceled := err.(CanceledError)
	require.True(t, canceled, "Expected rpc.CanceledError")

	ok := c.Finish(nil)
	require.False(t, ok, "Expected c.Finish to fail")

	closed := d.Close(nil)
	<-closed
}

func TestDispatchCanceledAfterResult(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	d, calls, done := dispatchTestCallWithContext(t, ctx)

	c := calls.retrieveCall(0)
	require.NotNil(t, c, "Expected c not to be nil")

	ok := c.Finish(nil)
	require.True(t, ok, "Expected c.Finish to succeed")

	cancel()

	err := <-done
	require.Nil(t, err, "Expected no error")

	closed := d.Close(nil)
	<-closed
}

func TestDispatchEOF(t *testing.T) {
	d, _, done := dispatchTestCall(t)

	closed := d.Close(nil)
	<-closed
	err := <-done
	require.Equal(t, io.EOF, err, "Expected EOF")
}

func TestDispatchCallAfterClose(t *testing.T) {
	d, calls, done := dispatchTestCall(t)

	c := calls.retrieveCall(0)
	c.Finish(nil)

	err := <-done
	closed := d.Close(nil)
	<-closed

	done = runInBg(func() error {
		return d.Call(context.Background(), "whatever", new(interface{}), new(interface{}), nil)
	})
	err = <-done
	require.Equal(t, io.EOF, err)
}
