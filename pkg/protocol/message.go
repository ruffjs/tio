package protocol

type ControlMessage struct {
	Type string `json:"t"`
	ID   string `json:"id,omitempty"`
	Data any    `json:"d,omitempty"`
}

const (
	MsgTypeReport = "report"
	MsgTypeGet    = "get"
	MsgTypeSet    = "set"
	MsgTypeCall   = "call"
	MsgTypeReply  = "reply"
)
