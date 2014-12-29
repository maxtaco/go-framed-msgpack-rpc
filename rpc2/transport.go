package rpc2

type Transporter interface {
	RawWrite([]byte) error
	PacketizerError(error)
}
