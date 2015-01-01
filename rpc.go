package framed_msgpack_rpc

//
// Most code borrowed from: github.com/ugorji/go/codec
// with small changes for framing & for single args (rather than arrays of 1)
//

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/ugorji/go/codec"
	"io"
	"io/ioutil"
	"net/rpc"
	"sync"
)

// -------------------------------------

// rpcCodec defines the struct members and common methods.
type rpcCodec struct {
	rwc     io.ReadWriteCloser
	dec     *codec.Decoder
	enc     *codec.Encoder
	benc    *codec.Encoder
	bw      *bufio.Writer
	br      *bufio.Reader
	mu      sync.Mutex
	cls     bool
	h       codec.Handle
	byteBuf *bytes.Buffer
}

func (c *rpcCodec) encodeToBytes(obj interface{}) []byte {
	c.benc.Encode(obj)
	b, _ := ioutil.ReadAll(c.byteBuf)
	return b
}

func newRPCCodec(conn io.ReadWriteCloser, h codec.Handle) rpcCodec {
	bw := bufio.NewWriter(conn)
	br := bufio.NewReader(conn)
	bb := new(bytes.Buffer)
	return rpcCodec{
		rwc:     conn,
		bw:      bw,
		br:      br,
		enc:     codec.NewEncoder(bw, h),
		dec:     codec.NewDecoder(br, h),
		benc:    codec.NewEncoder(bb, h),
		h:       h,
		byteBuf: bb,
	}
}

func (c *rpcCodec) BufferedReader() *bufio.Reader {
	return c.br
}

func (c *rpcCodec) BufferedWriter() *bufio.Writer {
	return c.bw
}

func (c *rpcCodec) write(obj interface{}) (err error) {
	if c.cls {
		return io.EOF
	}
	if err = c.enc.Encode(obj); err != nil {
		return
	}
	return c.bw.Flush()
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
	ret := c.read(body)
	return ret
}

//--------------------------------------------------

type msgpackSpecRpcCodec struct {
	rpcCodec
	framed bool
}

func (c *msgpackSpecRpcCodec) framedWrite(obj interface{}) (err error) {
	b2 := c.encodeToBytes(obj)
	l := len(b2)
	b1 := c.encodeToBytes(l)

	if c.cls {
		return io.EOF
	} else if _, err = c.bw.Write(b1); err != nil {
		return err
	} else if _, err = c.bw.Write(b2); err != nil {
		return err
	} else {
		err = c.bw.Flush()
	}
	return err
}

func (c *msgpackSpecRpcCodec) maybeFramedWrite(obj interface{}) error {
	if c.framed {
		return c.framedWrite(obj)
	} else {
		return c.write(obj)
	}
}

// /////////////// Spec RPC Codec ///////////////////
func (c *msgpackSpecRpcCodec) WriteRequest(r *rpc.Request, body interface{}) error {
	r2 := []interface{}{0, uint32(r.Seq), r.ServiceMethod, body}
	return c.maybeFramedWrite(r2)
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
	return c.maybeFramedWrite(r2)
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

	// Don't pass an array with just argument --- pass the arg itself;
	// this is a big difference from the underlying library.
	// bodyArr := []interface{}{body}
	return c.read(&body)
}

func (c *msgpackSpecRpcCodec) parseCustomHeader(expectTypeByte byte, msgid *uint64, methodOrError *string) (err error) {

	if c.cls {
		return io.EOF
	}

	if c.framed {
		var frameByte int
		if err = c.read(&frameByte); err != nil {
			return err
		}
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

func (x msgpackSpecRpc) ServerCodec(conn io.ReadWriteCloser, h codec.Handle, framed bool) rpc.ServerCodec {
	return &msgpackSpecRpcCodec{newRPCCodec(conn, h), framed}
}

func (x msgpackSpecRpc) ClientCodec(conn io.ReadWriteCloser, h codec.Handle, framed bool) rpc.ClientCodec {
	return &msgpackSpecRpcCodec{newRPCCodec(conn, h), framed}
}
