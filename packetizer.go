package rpc

type packetizer struct {
	dispatch  dispatcher
	transport transporter
}

func newPacketizer(d dispatcher, t transporter) *packetizer {
	return &packetizer{
		dispatch:  d,
		transport: t,
	}
}

func (p *packetizer) getFrame() (int, error) {
	var l int

	err := p.transport.Decode(&l)

	return l, err
}

func (p *packetizer) Clear() {
	p.dispatch = nil
	p.transport = nil
}

func (p *packetizer) getMessage(l int) (err error) {
	var b byte

	if b, err = p.transport.ReadByte(); err != nil {
		return err
	}
	nb := int(b)

	if nb >= 0x91 && nb <= 0x9f {
		err = p.dispatch.Dispatch(nb - 0x90)
	} else {
		err = NewPacketizerError("wrong message structure prefix (%d)", nb)
	}

	return err
}

func (p *packetizer) packetizeOne() (err error) {
	var n int
	if n, err = p.getFrame(); err == nil {
		err = p.getMessage(n)
	}
	return
}

func (p *packetizer) Packetize() (err error) {
	for err == nil {
		err = p.packetizeOne()
	}
	return
}
