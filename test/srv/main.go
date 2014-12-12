package main

import (
    "log"
    "net"
    "fmt"
    "net/rpc"
    "github.com/ugorji/go/codec"
    fmprpc "github.com/maxtaco/go-framed-msgpack-rpc"
)

//----------------------------------------------------------------------
// This was generated from the keybase/protocol library, on
// the test_rpc_1 branch.
//


type AddArg struct {
	A int `codec:"a"`
	B int `codec:"b"`
}

type AddRes struct {
	C int `codec:"c"`
}

type BrokenArg struct {}
type BrokenRes struct {}


type ArithInterface interface {
	Add(arg *AddArg, res *AddRes) error
    Broken(arg *BrokenArg, res *BrokenRes) error
}

func RegisterArith(server *rpc.Server, i ArithInterface) error {
	return server.RegisterName("test.1.arith", i)
}

//
// end autogen
//----------------------------------------------------------------------

type Arith struct {}

func (a Arith) Add(arg *AddArg, res *AddRes) error {
	res.C = arg.A + arg.B
	return nil
}

func (a Arith) Broken(arg *BrokenArg, res *BrokenRes) error {
    return fmt.Errorf("broken is broken")
}

func startServer() {
    arith := new(Arith)

    server := rpc.NewServer()
    RegisterArith(server, arith)
    var mh codec.MsgpackHandle

    l, e := net.Listen("tcp", ":8222")
    if e != nil {
        log.Fatal("listen error:", e)
    }

    for {
        conn, err := l.Accept()
        if err != nil {
            log.Fatal(err)
        }
        rpcCodec := fmprpc.MsgpackSpecRpc.ServerCodec(conn, &mh, true)

        go server.ServeCodec(rpcCodec)
    }
}


func main() {
	startServer()
}
