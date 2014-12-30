package rpc2

type Server struct {
	xp *Transport
}

func NewServer(xp *Transport) *Server {
	return &Server{xp}
}

func (s *Server) Register(p Protocol) (err error) {
	return s.xp.dispatcher.RegisterProtocol(p)
}
