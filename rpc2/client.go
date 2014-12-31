package rpc2

import ()

type Client struct {
	xp *Transport
}

func NewClient(xp *Transport) *Client {
	return &Client{xp}
}

func (c *Client) Call(method string, arg interface{}, res interface{}) (err error) {
	var d Dispatcher
	if d, err = c.xp.GetDispatcher(); err == nil {
		err = d.Call(method, arg, res)
	}
	return
}
