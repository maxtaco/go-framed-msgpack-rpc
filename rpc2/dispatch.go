package rpc2

type Dispatcher interface {
	Dispatch(msg Message) error
}
