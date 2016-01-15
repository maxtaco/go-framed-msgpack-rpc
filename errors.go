package rpc

import (
	"fmt"
)

type RecoverableError interface {
	error
	CanRecover() bool
}

type BasicRPCError struct{}

func (BasicRPCError) CanRecover() bool { return false }

type RPCErrorWrapper struct {
	BasicRPCError
	error
}

func newRPCErrorWrapper(err error) RPCErrorWrapper {
	return RPCErrorWrapper{error: err}
}

type PacketizerError struct {
	BasicRPCError
	msg string
}

func (p PacketizerError) Error() string {
	return "packetizer error: " + p.msg
}

func NewPacketizerError(d string, a ...interface{}) PacketizerError {
	return PacketizerError{msg: fmt.Sprintf(d, a...)}
}

type DispatcherError struct {
	BasicRPCError
	msg string
}

func (p DispatcherError) Error() string {
	return "dispatcher error: " + p.msg
}

func NewDispatcherError(d string, a ...interface{}) DispatcherError {
	return DispatcherError{msg: fmt.Sprintf(d, a...)}
}

type ReceiverError struct {
	BasicRPCError
	msg string
}

func (p ReceiverError) Error() string {
	return "dispatcher error: " + p.msg
}

func NewReceiverError(d string, a ...interface{}) ReceiverError {
	return ReceiverError{msg: fmt.Sprintf(d, a...)}
}

type MethodNotFoundError struct {
	BasicRPCError
	p string
	m string
}

func newMethodNotFoundError(p, m string) MethodNotFoundError {
	return MethodNotFoundError{
		p: p,
		m: m,
	}
}

func (m MethodNotFoundError) Error() string {
	return fmt.Sprintf("method '%s' not found in protocol '%s'", m.m, m.p)
}

type ProtocolNotFoundError struct {
	BasicRPCError
	p string
}

func newProtocolNotFoundError(p string) ProtocolNotFoundError {
	return ProtocolNotFoundError{p: p}
}

func (p ProtocolNotFoundError) Error() string {
	return "protocol not found: " + p.p
}

type AlreadyRegisteredError struct {
	BasicRPCError
	p string
}

func newAlreadyRegisteredError(p string) AlreadyRegisteredError {
	return AlreadyRegisteredError{p: p}
}

func (a AlreadyRegisteredError) Error() string {
	return a.p + ": protocol already registered"
}

type TypeError struct {
	BasicRPCError
	p string
}

func (t TypeError) Error() string {
	return t.p
}

func NewTypeError(expected, actual interface{}) TypeError {
	return TypeError{p: fmt.Sprintf("Invalid type for arguments. Expected: %T, actual: %T", expected, actual)}
}

type CallNotFoundError struct {
	BasicRPCError
	seqno seqNumber
}

func newCallNotFoundError(s seqNumber) CallNotFoundError {
	return CallNotFoundError{seqno: s}
}

func (c CallNotFoundError) Error() string {
	return fmt.Sprintf("Call not found for sequence number %d", c.seqno)
}

func (CallNotFoundError) CanRecover() bool { return true }

type NilResultError struct {
	BasicRPCError
	seqno seqNumber
}

func (c NilResultError) Error() string {
	return fmt.Sprintf("Nil result supplied for sequence number %d", c.seqno)
}

type RPCDecodeError struct {
	typ MethodType
	len int
	err error
}

func (r RPCDecodeError) CanRecover() bool {
	if err, ok := r.err.(RecoverableError); ok {
		return err.CanRecover()
	}
	return false
}

func (r RPCDecodeError) Error() string {
	return fmt.Sprintf("RPC error: type %d, length %d, error: %v", r.typ, r.len, r.err)
}

func newRPCDecodeError(t MethodType, l int, e error) RPCDecodeError {
	return RPCDecodeError{
		typ: t,
		len: l,
		err: e,
	}
}

func newRPCMessageFieldDecodeError(i int, err error) error {
	return fmt.Errorf("error decoding message field at position %d, error: %v", i, err)
}
