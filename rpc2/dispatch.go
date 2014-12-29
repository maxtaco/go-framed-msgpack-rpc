package rpc2

type Dispatcher interface {
	Dispatch(msg Message) error
	DispatchTriple(t Transporter) error
	DispatchQuad(t Transporter) error
}
