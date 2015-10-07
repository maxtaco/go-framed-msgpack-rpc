package rpc2

type Server struct {
	xp        *Transport
	wrapError WrapErrorFunc
}

func NewServer(xp *Transport, f WrapErrorFunc) *Server {
	return &Server{xp, f}
}

func (s *Server) Register(p Protocol) (err error) {
	p.WrapError = s.wrapError
	dispatcher, err := s.xp.GetDispatcher()
	if err != nil {
		return err
	}
	return dispatcher.RegisterProtocol(p)
}

// RegisterEOFHook registers a callback that's called when there's
// and EOF condition on the underlying channel.
func (s *Server) RegisterEOFHook(h EOFHook) error {
	dispatcher, err := s.xp.GetDispatcher()
	if err != nil {
		return err
	}
	return dispatcher.RegisterEOFHook(h)
}

func (s *Server) Run(bg bool) error {
	return s.xp.run(bg)
}
