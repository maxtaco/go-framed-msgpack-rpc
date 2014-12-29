package rpc2

import (
	"fmt"
)

type PacketizerError struct {
	msg string
}

func (p PacketizerError) Error() string {
	return "packetizer error: " + p.msg
}

func NewPacketizerError(d string, a ...interface{}) PacketizerError {
	return PacketizerError{fmt.Sprintf(d, a...)}
}

type DispatcherError struct {
	msg string
}

func (p DispatcherError) Error() string {
	return "dispatcher error: " + p.msg
}

func NewDispatcherError(d string, a ...interface{}) DispatcherError {
	return DispatcherError{fmt.Sprintf(d, a...)}
}
