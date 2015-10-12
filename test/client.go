package main

import (
	"golang.org/x/net/context"
)

type GenericClient interface {
	Call(ctx context.Context, method string, arg interface{}, res interface{}) error
	Notify(ctx context.Context, method string, arg interface{}) error
}

//---------------------------------------------------------------------

type TestClient struct {
	GenericClient
}

func (a TestClient) Add(ctx context.Context, arg AddArgs) (ret int, err error) {
	err = a.Call(ctx, "test.1.testp.add", arg, &ret)
	return
}

func (a TestClient) Broken() (err error) {
	err = a.Call(nil, "test.1.testp.broken", nil, nil)
	return
}

func (a TestClient) UpdateConstants(ctx context.Context, arg Constants) (err error) {
	err = a.Notify(ctx, "test.1.testp.updateConstants", arg)
	return
}

func (a TestClient) GetConstants(ctx context.Context) (ret Constants, err error) {
	err = a.Call(ctx, "test.1.testp.GetConstants", nil, &ret)
	return
}

func (a TestClient) LongCall(ctx context.Context) (ret int, err error) {
	err = a.Call(ctx, "test.1.testp.LongCall", nil, &ret)
	return
}

//---------------------------------------------------------------------

type Client struct {
	port int
}
