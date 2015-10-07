package rpc

type message struct {
	t        transporter
	nFields  int
	nDecoded int
}

func (m *message) Decode(i interface{}) (err error) {
	err = m.t.Decode(i)
	if err == nil {
		m.nDecoded++
	}
	return err
}

func (m *message) makeDecodeNext(debugHook func(interface{})) DecodeNext {
	// Reserve the next object
	m.t.Lock()
	return func(i interface{}) error {
		ret := m.Decode(i)
		if debugHook != nil {
			debugHook(i)
		}
		m.t.Unlock()
		return ret
	}
}
