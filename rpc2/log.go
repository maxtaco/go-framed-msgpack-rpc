package rpc2

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"time"
)

type Profiler interface {
	Stop()
}

type LogInterface interface {
	TransportStart()
	TransportError(error)
	ServerCall(int, string, error, interface{})
	ServerReply(int, string, error, interface{})
	StartProfiler(format string, args ...interface{}) Profiler
	UnexpectedReply(int)
	Warning(format string, args ...interface{})
}

type LogFactory interface {
	NewLog(net.Addr) LogInterface
}

type LogOutput interface {
	Error(s string)
	Warning(s string)
	Info(s string)
	Debug(s string)
	Profile(s string)
}

type LogOptions interface {
	ShowAddress() bool
	ShowArg() bool
	ShowResult() bool
	DoProfile() bool
}

//-------------------------------------------------

type SimpleLogFactory struct {
	out  LogOutput
	opts LogOptions
}

type SimpleLog struct {
	addr net.Addr
	out  LogOutput
	opts LogOptions
}

type SimpleLogOutput struct{}
type SimpleLogOptions struct{}

func (so SimpleLogOutput) Info(s string)    { fmt.Fprintf(os.Stdout, "[I] %s\n", s) }
func (so SimpleLogOutput) Error(s string)   { fmt.Fprintf(os.Stdout, "[E] %s\n", s) }
func (so SimpleLogOutput) Debug(s string)   { fmt.Fprintf(os.Stdout, "[D] %s\n", s) }
func (so SimpleLogOutput) Warning(s string) { fmt.Fprintf(os.Stdout, "[W] %s\n", s) }
func (so SimpleLogOutput) Profile(s string) { fmt.Fprintf(os.Stdout, "[P] %s\n", s) }

func (so SimpleLogOptions) ShowAddress() bool { return true }
func (so SimpleLogOptions) ShowArg() bool     { return true }
func (so SimpleLogOptions) ShowResult() bool  { return true }
func (so SimpleLogOptions) DoProfile() bool   { return true }

func NewSimpleLogFactory(out LogOutput, opts LogOptions) SimpleLogFactory {
	if out == nil {
		out = SimpleLogOutput{}
	}
	if opts == nil {
		opts = SimpleLogOptions{}
	}
	ret := SimpleLogFactory{out, opts}
	return ret
}

func (s SimpleLogFactory) NewLog(a net.Addr) LogInterface {
	ret := SimpleLog{a, s.out, s.opts}
	ret.TransportStart()
	return ret
}

func AddrToString(addr net.Addr) string {
	if addr == nil {
		return "-"
	} else {
		c := addr.String()
		if len(c) == 0 {
			c = "localhost"
		}
		return addr.Network() + "://" + c
	}
}

func (l SimpleLog) TransportStart() {
	l.out.Info(l.msg(true, "New connection"))
}

func (l SimpleLog) TransportError(e error) {
	if e != io.EOF {
		l.out.Error(l.msg(true, "Error in transport: %s", e.Error()))
	} else {
		l.out.Info(l.msg(true, "EOF"))
	}
	return
}

func (s SimpleLog) ServerReply(q int, meth string, err error, res interface{}) {
	s.trace("reply", "result", s.opts.ShowResult(), q, meth, err, res)
}
func (s SimpleLog) ServerCall(q int, meth string, err error, res interface{}) {
	s.trace("call", "arg", s.opts.ShowArg(), q, meth, err, res)
}

func (s SimpleLog) trace(which string, objname string, verbose bool, q int, meth string, err error, obj interface{}) {
	fmts := "%s(%d): method=%s; err=%s"
	var es string
	if err == nil {
		es = "null"
	} else {
		es = err.Error()
	}
	args := []interface{}{which, q, meth, es}
	if verbose {
		fmts += "; %s=%s"
		eb, err := json.Marshal(obj)
		var es string
		if err != nil {
			es = fmt.Sprintf(`{"error": "%s"}`, err.Error())
		} else {
			es = string(eb)
		}
		args = append(args, objname)
		args = append(args, es)
	}
	s.out.Debug(s.msg(false, fmts, args...))
}

func (s SimpleLog) StartProfiler(format string, args ...interface{}) Profiler {
	if s.opts.DoProfile() {
		return SimpleProfiler{
			start: time.Now(),
			msg:   fmt.Sprintf(format, args...),
			log:   s,
		}
	} else {
		return nil
	}
}

func (s SimpleLog) UnexpectedReply(seqno int) {
	s.out.Warning(s.msg(false, "Unexpected seqno %d in incoming reply", seqno))
}

func (s SimpleLog) Warning(format string, args ...interface{}) {
	s.out.Warning(s.msg(false, format, args...))
}

func (l SimpleLog) msg(force bool, format string, args ...interface{}) string {
	m1 := fmt.Sprintf(format, args...)
	if l.opts.ShowAddress() || force {
		m2 := fmt.Sprintf("{%s} %s", AddrToString(l.addr), m1)
		m1 = m2
	}
	return m1
}

type SimpleProfiler struct {
	start time.Time
	msg   string
	log   SimpleLog
}

func (s SimpleProfiler) Stop() {
	stop := time.Now()
	diff := stop.Sub(s.start)
	s.log.out.Profile(s.log.msg(false, "%s ran in %dms", s.msg, diff/time.Millisecond))
}
