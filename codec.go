package rpc

type Decoder interface {
	Decode(interface{}) error
}
