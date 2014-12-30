package rpc2

import ()

type Errors struct {
	v []error
}

func (es *Errors) Push(e error) bool {
	if e != nil {
		es.v = append(es.v, e)
		return false
	} else {
		return true
	}
}

func (e *Errors) Error() error {
	if len(e.v) > 0 {
		return e.v[0]
	} else {
		return nil
	}
}

func MakeMethodName(prot string, method string) string {
	if len(prot) == 0 {
		return method
	} else {
		return prot + "." + method
	}
}
