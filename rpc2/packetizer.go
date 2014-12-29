package rpc2

const (
	state_frame = iota
	state_data  = iota
)

type Packetizer struct {
	state     int
	dispatch  Dispatcher
	transport Transporter
}

func NewPacketizer(d Dispatcher, t Transporter) *Packetizer {
	return &Packetizer{
		state:     state_frame,
		dispatch:  d,
		transport: t,
	}
}
