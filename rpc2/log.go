package rpc2

import (
	"fmt"
	"github.com/op/go-logging"
	"os"
	"time"
)

type Logger interface {
	Critical(format string, args ...interface{})
	Debug(format string, args ...interface{})
	Error(format string, args ...interface{})
	Info(format string, args ...interface{})
	Warning(format string, args ...interface{})
	StartProfiler(format string, args ...interface{}) Profiler
}

type Profiler interface {
	Stop()
}

type SimpleLog struct {
	logging.Logger
}

func (s *SimpleLog) Init() {
	logBackend := logging.NewLogBackend(os.Stderr, "", 0)
	logging.SetBackend(logBackend)
	logging.SetLevel(logging.INFO, "rpc2")
}

func NewSimpleLog() *SimpleLog {
	log := logging.MustGetLogger("rpc2")
	ret := &SimpleLog{*log}
	ret.Init()
	return ret
}

type SimpleProfiler struct {
	start time.Time
	msg   string
	log   Logger
}

func (s *SimpleLog) StartProfiler(format string, args ...interface{}) Profiler {
	return &SimpleProfiler{
		start: time.Now(),
		msg:   fmt.Sprintf(format, args...),
		log:   s,
	}
}

func (s *SimpleProfiler) Stop() {
	stop := time.Now()
	diff := stop.Sub(s.start)
	s.log.Info("[P] %s ran in %dms", s.msg, diff/time.Millisecond)
}
