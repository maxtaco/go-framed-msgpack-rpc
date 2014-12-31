package rpc2

import ()

type Message struct {
	t        Transporter
	nFields  int
	nDecoded int
}

func (m *Message) Decode(i interface{}) (err error) {
	err = m.t.Decode(i)
	if err == nil {
		m.nDecoded++
	}
	return err
}

func (m *Message) WrapError(err error) interface{} {
	return m.t.WrapError(err)
}

func (m *Message) DecodeError() (app error, dispatch error) {
	app, dispatch = m.t.UnwrapError(m.makeDecodeNext())
	return
}

func (m *Message) Encode(i interface{}) error {
	return m.t.Encode(i)
}

func (m *Message) decodeToNull() error {
	var err error
	for err == nil && m.nDecoded < m.nFields {
		var i interface{}
		m.Decode(&i)
	}
	return err
}

func (m *Message) makeDecodeNext() DecodeNext {
	// Reserve the next object
	m.t.ReadLock()
	return func(i interface{}) error {
		ret := m.Decode(i)
		m.t.ReadUnlock()
		return ret
	}
}
