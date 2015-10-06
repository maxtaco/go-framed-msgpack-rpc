package main

import (
	"fmt"
	"net"
	"testing"
	"time"

	rpc "github.com/keybase/go-framed-msgpack-rpc"
	"github.com/stretchr/testify/assert"
)

func TestProtocol(t *testing.T) {
	port := 8089
	server := &Server{port: 8089}

	serverReady := make(chan struct{})
	go func() {
		err := server.Run(serverReady)
		assert.Nil(t, err, "a server error occurred")
	}()
	<-serverReady

	c, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return
	}

	xp := rpc.NewTransport(c, nil, nil)
	cli := ArithClient{GenericClient: rpc.NewClient(xp, nil)}

	B := 34
	for A := 10; A < 23; A += 2 {
		var res int
		if res, err = cli.Add(AddArgs{A: A, B: B}); err != nil {
			return
		}
		assert.Equal(t, A+B, res, "Result should be the two parameters added together")
	}

	err = cli.Broken()
	assert.Error(t, err, "Called nonexistent method, expected error")

	pi := 31415

	if err = cli.UpdateConstants(Constants{Pi: pi}); err != nil {
		t.Fatalf("Unexpected error on notify: %v", err)
	}
	time.Sleep(3 * time.Millisecond)
	var constants Constants
	if constants, err = cli.GetConstants(); err != nil {
		t.Fatalf("Unexpected error on GetConstants: %v", err)
	} else {
		assert.Equal(t, pi, constants.Pi, "we set the constant properly via Notify")
	}
}
