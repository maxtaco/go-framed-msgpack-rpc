
package framed_msgpack_rpc

//
// Most code borrowed from
//	"github.com/ugorji/go/codec"
// with small changes for framing
//

import (
	"github.com/ugorji/go/codec"
	"bufio"
	"io"
	"sync"
	"net/rpc"
	"fmt"
)


// -------------------------------------

// rpcCodec defines the struct members and common methods.
type rpcCodec struct {
	rwc io.ReadWriteCloser
	dec *codec.Decoder
	enc *codec.Encoder
	bw  *bufio.Writer
	br  *bufio.Reader
	mu  sync.Mutex
	cls bool
	h   codec.Handle
}

func newRPCCodec(conn io.ReadWriteCloser, h codec.Handle) rpcCodec {
	bw := bufio.NewWriter(conn)
	br := bufio.NewReader(conn)
	return rpcCodec{
		rwc: conn,
		bw:  bw,
		br:  br,
		enc: codec.NewEncoder(bw, h),
		dec: codec.NewDecoder(br, h),
		h:   h,
	}
}

func (c *rpcCodec) BufferedReader() *bufio.Reader {
	return c.br
}

func (c *rpcCodec) BufferedWriter() *bufio.Writer {
	return c.bw
}

func (c *rpcCodec) write(obj1, obj2 interface{}, writeObj2, doFlush bool) (err error) {
	if c.cls {
		return io.EOF
	}
	if err = c.enc.Encode(obj1); err != nil {
		return
	}
	if writeObj2 {
		if err = c.enc.Encode(obj2); err != nil {
			return
		}
	}
	if doFlush {
		return c.bw.Flush()
	}
	return
}

func (c *rpcCodec) read(obj interface{}) (err error) {
	if c.cls {
		return io.EOF
	}
	//If nil is passed in, we should still attempt to read content to nowhere.
	if obj == nil {
		var obj2 interface{}
		return c.dec.Decode(&obj2)
	}
	return c.dec.Decode(obj)
}

func (c *rpcCodec) Close() error {
	if c.cls {
		return io.EOF
	}
	c.cls = true
	return c.rwc.Close()
}

func (c *rpcCodec) ReadResponseBody(body interface{}) error {
	return c.read(body)
}

//--------------------------------------------------

type msgpackSpecRpcCodec struct {
	rpcCodec
}

// /////////////// Spec RPC Codec ///////////////////
func (c *msgpackSpecRpcCodec) WriteRequest(r *rpc.Request, body interface{}) error {
	// WriteRequest can write to both a Go service, and other services that do
	// not abide by the 1 argument rule of a Go service.
	// We discriminate based on if the body is a MsgpackSpecRpcMultiArgs
	var bodyArr []interface{}
	if m, ok := body.(codec.MsgpackSpecRpcMultiArgs); ok {
		bodyArr = ([]interface{})(m)
	} else {
		bodyArr = []interface{}{body}
	}
	r2 := []interface{}{0, uint32(r.Seq), r.ServiceMethod, bodyArr}
	return c.write(r2, nil, false, true)
}

func (c *msgpackSpecRpcCodec) WriteResponse(r *rpc.Response, body interface{}) error {
	var moe interface{}
	if r.Error != "" {
		moe = r.Error
	}
	if moe != nil && body != nil {
		body = nil
	}
	r2 := []interface{}{1, uint32(r.Seq), moe, body}
	return c.write(r2, nil, false, true)
}

func (c *msgpackSpecRpcCodec) ReadResponseHeader(r *rpc.Response) error {
	return c.parseCustomHeader(1, &r.Seq, &r.Error)
}

func (c *msgpackSpecRpcCodec) ReadRequestHeader(r *rpc.Request) error {
	return c.parseCustomHeader(0, &r.Seq, &r.ServiceMethod)
}

func (c *msgpackSpecRpcCodec) ReadRequestBody(body interface{}) error {
	if body == nil { // read and discard
		return c.read(nil)
	}
	bodyArr := []interface{}{body}
	return c.read(&bodyArr)
}

func (c *msgpackSpecRpcCodec) parseCustomHeader(expectTypeByte byte, msgid *uint64, methodOrError *string) (err error) {

	if c.cls {
		return io.EOF
	}

	// We read the response header by hand
	// so that the body can be decoded on its own from the stream at a later time.

	const fia byte = 0x94 //four item array descriptor value
	// Not sure why the panic of EOF is swallowed above.
	// if bs1 := c.dec.r.readn1(); bs1 != fia {
	// 	err = fmt.Errorf("Unexpected value for array descriptor: Expecting %v. Received %v", fia, bs1)
	// 	return
	// }
	var b byte
	b, err = c.br.ReadByte()
	if err != nil {
		return
	}
	if b != fia {
		err = fmt.Errorf("Unexpected value for array descriptor: Expecting %v. Received %v", fia, b)
		return
	}

	if err = c.read(&b); err != nil {
		return
	}
	if b != expectTypeByte {
		err = fmt.Errorf("Unexpected byte descriptor in header. Expecting %v. Received %v", expectTypeByte, b)
		return
	}
	if err = c.read(msgid); err != nil {
		return
	}
	if err = c.read(methodOrError); err != nil {
		return
	}
	return
}

//--------------------------------------------------

// msgpackSpecRpc is the implementation of Rpc that uses custom communication protocol
// as defined in the msgpack spec at https://github.com/msgpack-rpc/msgpack-rpc/blob/master/spec.md
type msgpackSpecRpc struct{}

// MsgpackSpecRpc implements Rpc using the communication protocol defined in
// the msgpack spec at https://github.com/msgpack-rpc/msgpack-rpc/blob/master/spec.md .
// Its methods (ServerCodec and ClientCodec) return values that implement RpcCodecBuffered.
var MsgpackSpecRpc msgpackSpecRpc

func (x msgpackSpecRpc) ServerCodec(conn io.ReadWriteCloser, h codec.Handle) rpc.ServerCodec {
	return &msgpackSpecRpcCodec{newRPCCodec(conn, h)}
}

func (x msgpackSpecRpc) ClientCodec(conn io.ReadWriteCloser, h codec.Handle) rpc.ClientCodec {
	return &msgpackSpecRpcCodec{newRPCCodec(conn, h)}
}


