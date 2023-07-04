package shadow

import (
	"context"
)

type Operation int

const (
	OpGet Operation = iota
	OpUpdate
)

type StateHandler interface {
	// ShadowGetReq the request to get device shadow
	ShadowGetReq(ctx context.Context) (<-chan GetReqMsg, error)
	// StateUpdateReq either update state desired or reported, not both the same time
	// State.Desired or State.Desired should be nil
	StateUpdateReq(ctx context.Context) (<-chan StateReqMsg, error)
	RejectedResp(ctx context.Context, resp ErrRespMsg) error
	AcceptedResp(ctx context.Context, resp StateAcceptedRespMsg) error
	StateDeltaNotify(ctx context.Context, notice DeltaStateNoticeMsg) error
	StateUpdatedNotify(ctx context.Context, notice StateUpdatedNoticeMsg) error
}

type GetReqMsg struct {
	ThingId string
	Req     GetReq
}

type StateReqMsg struct {
	ThingId string
	Req     StateReq
}

type ErrRespMsg struct {
	ThingId string
	Op      Operation
	Resp    ErrResp
}

type StateAcceptedRespMsg struct {
	ThingId string
	Op      Operation
	Resp    StateAcceptedResp
}

type DeltaStateNoticeMsg struct {
	ThingId string
	Notice  DeltaStateNotice
}

type StateUpdatedNoticeMsg struct {
	ThingId string
	Notice  StateUpdatedNotice
}
