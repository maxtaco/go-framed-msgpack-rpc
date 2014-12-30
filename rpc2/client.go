package rpc2

type Client struct {
	xp       *Transport
	protocol string
}

func NewClient(xp *Transport, protocol string) *Client {
	return &Client{xp, protocol}
}

func (c *Client) Call(method string, arg interface{}) (ret DecodeNext, err error) {
	var d Dispatcher
	if d, err = c.xp.GetDispatcher(); err == nil {
		ret, err = d.Call(MakeMethodName(c.protocol, method), arg)
	}
	return
}
