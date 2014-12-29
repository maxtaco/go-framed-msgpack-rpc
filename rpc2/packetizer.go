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

func (p *Packetizer) getFrame() (int, error) {
	var l int
	err := p.transport.Decode(&l)
	return l, err
}

func (p *Packetizer) getMessage(l int) (err error) {
	var b byte
	if b, err = p.transport.ReadByte(); err != nil {
		return err
	}
	nb := int(b)

	switch nb {
	case 0x93:
		err = p.dispatch.DispatchTriple(p.transport)
	case 0x94:
		err = p.dispatch.DispatchQuad(p.transport)
	default:
		err = NewPacketizerError("wrong number of fields in message (%d)", nb)
	}

	return err
}

func (p *Packetizer) packetizeOne() (err error) {
	var n int
	if n, err = p.getFrame(); err == nil {
		err = p.getMessage(n)
	}
	return
}

func (p *Packetizer) Packetize() (err error) {
	for err == nil {
		err = p.packetizeOne()
	}
	return
}
