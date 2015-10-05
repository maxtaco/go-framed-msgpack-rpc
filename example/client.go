package main

import (
	"fmt"
	"net"
	"time"

	rpc "github.com/keybase/go-framed-msgpack-rpc"
)

type GenericClient interface {
	Call(method string, arg interface{}, res interface{}) error
	Notify(method string, arg interface{}) error
}

//---------------------------------------------------------------------

type ArithClient struct {
	GenericClient
}

func (a ArithClient) Add(arg AddArgs) (ret int, err error) {
	err = a.Call("test.1.arith.add", arg, &ret)
	return
}

func (a ArithClient) Broken() (err error) {
	err = a.Call("test.1.arith.broken", nil, nil)
	return
}

func (a ArithClient) UpdateConstants(arg Constants) (err error) {
	err = a.Notify("test.1.arith.updateConstants", arg)
	return
}

func (a ArithClient) GetConstants() (ret Constants, err error) {
	err = a.Call("test.1.arith.GetConstants", nil, &ret)
	return
}

//---------------------------------------------------------------------

type Client struct {
	port int
}

func (s *Client) Run() (err error) {
	var c net.Conn
	if c, err = net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", s.port)); err != nil {
		return
	}

	xp := rpc.NewTransport(c, nil, nil)
	cli := ArithClient{GenericClient: rpc.NewClient(xp, nil)}

	for A := 10; A < 23; A += 2 {
		var res int
		if res, err = cli.Add(AddArgs{A: A, B: 34}); err != nil {
			return
		}
		fmt.Printf("result is -> %v\n", res)
	}

	err = cli.Broken()
	fmt.Printf("for broken: %v\n", err)

	if err = cli.UpdateConstants(Constants{Pi: 314}); err != nil {
		return err
	}
	time.Sleep(3 * time.Millisecond)
	var constants Constants
	if constants, err = cli.GetConstants(); err != nil {
		return
	} else {
		fmt.Printf("constants -> %v\n", constants)
	}

	return nil
}
