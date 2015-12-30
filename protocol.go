package rpc

import (
	"sync"

	"golang.org/x/net/context"
)

type ServeHandlerDescription struct {
	MakeArg    func() interface{}
	Handler    func(ctx context.Context, arg interface{}) (ret interface{}, err error)
	MethodType MethodType
}

type MethodType int

const (
	MethodCall     MethodType = 0
	MethodResponse            = 1
	MethodNotify              = 2
	MethodCancel              = 3
)

type ErrorUnwrapper interface {
	MakeArg() interface{}
	UnwrapError(arg interface{}) (appError error, dispatchError error)
}

type Protocol struct {
	Name      string
	Methods   map[string]ServeHandlerDescription
	WrapError WrapErrorFunc
}

type protocolMap map[string]Protocol

type seqNumber int

type protocolHandler struct {
	protocols protocolMap
	wef       WrapErrorFunc
	mtx       sync.Mutex
}

func newProtocolHandler(wef WrapErrorFunc) *protocolHandler {
	return &protocolHandler{
		wef:       wef,
		protocols: make(protocolMap),
	}
}

func (h *protocolHandler) registerProtocol(p Protocol) error {
	h.mtx.Lock()
	defer h.mtx.Unlock()

	if _, found := h.protocols[p.Name]; found {
		return AlreadyRegisteredError{p.Name}
	}
	h.protocols[p.Name] = p
	return nil
}

func (h *protocolHandler) findServeHandler(name string) (*ServeHandlerDescription, WrapErrorFunc, error) {
	h.mtx.Lock()
	defer h.mtx.Unlock()

	p, m := splitMethodName(name)
	prot, found := h.protocols[p]
	if !found {
		return nil, h.wef, ProtocolNotFoundError{p}
	}
	srv, found := prot.Methods[m]
	if !found {
		return nil, h.wef, MethodNotFoundError{p, m}
	}
	return &srv, prot.WrapError, nil
}

func (h *protocolHandler) getArg(name string) (interface{}, error) {
	handler, _, err := h.findServeHandler(name)
	if err != nil {
		return nil, err
	}
	return handler.MakeArg(), nil
}
