package protocol

// ShadowState wraps desired and reported for shadow_get_reply.
type ShadowState struct {
	Desired  map[string]any `json:"desired"`
	Reported map[string]any `json:"reported"`
}

// ShadowGetReply is the payload for tio/{thingId}/down/shadow_get_reply.
type ShadowGetReply struct {
	Code    int          `json:"code"`
	Version int64        `json:"version,omitempty"`
	Message string       `json:"message,omitempty"`
	State   *ShadowState `json:"state,omitempty"`
}

// ShadowUpdateReq is the payload for tio/{thingId}/up/shadow_update.
type ShadowUpdateReq struct {
	Version int64          `json:"version,omitempty"` // 0 = no optimistic lock
	State   map[string]any `json:"state"`
}

// ShadowUpdateReply is the payload for tio/{thingId}/down/shadow_update_reply.
type ShadowUpdateReply struct {
	Code    int    `json:"code"`
	Version int64  `json:"version,omitempty"`
	Message string `json:"message,omitempty"`
}

// ShadowDesired is the payload for tio/{thingId}/down/shadow_desired.
type ShadowDesired struct {
	Version int64          `json:"version"`
	State   map[string]any `json:"state"`
}

// MethodReq is the payload for tio/{thingId}/down/method_req.
type MethodReq struct {
	ID     string `json:"id"`
	Method string `json:"method"`
	Data   any    `json:"data,omitempty"`
}

// MethodResp is the payload for tio/{thingId}/up/method_resp.
type MethodResp struct {
	ID      string `json:"id"`
	Code    int    `json:"code"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
}

// NtpReq is the payload for tio/{thingId}/up/ntp_req.
type NtpReq struct {
	ClientSendTime int64 `json:"clientSendTime"`
}

// NtpResp is the payload for tio/{thingId}/down/ntp_resp.
type NtpResp struct {
	Code           int    `json:"code"`
	Message        string `json:"message,omitempty"`
	ClientSendTime int64  `json:"clientSendTime,omitempty"`
	ServerRecvTime int64  `json:"serverRecvTime,omitempty"`
	ServerSendTime int64  `json:"serverSendTime,omitempty"`
}

// SimpleInvokeResult carries the device method response code and data.
type SimpleInvokeResult struct {
	Code    int
	Data    any
	Message string
}
