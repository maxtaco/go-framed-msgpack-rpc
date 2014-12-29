package rpc2

type BaseMessage struct {
	msgType int
	seqno   int
}

func (b BaseMessage) GetSeqno() int   { return b.seqno }
func (b BaseMessage) GetMsgType() int { return b.msgType }

type Message interface {
	GetSeqno() int
	GetMsgType() int
}
