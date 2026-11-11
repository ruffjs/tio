package protocol

// ShadowGetReq selects the state returned by shadow_get; an empty type means all.
type ShadowGetReq struct {
	Type string `json:"type,omitzero"`
}

// ShadowState wraps desired and reported for shadow_get_reply.
type ShadowState struct {
	Desired  map[string]any `json:"desired,omitzero"`
	Reported map[string]any `json:"reported,omitzero"`
}

// ShadowGetReply is the payload for tio/{thingId}/down/shadow_get_reply.
type ShadowGetReply struct {
	Code    int          `json:"code"`
	Version int64        `json:"version,omitzero"`
	Message string       `json:"message,omitzero"`
	State   *ShadowState `json:"state,omitzero"`
}

// ShadowUpdateReq is the payload for tio/{thingId}/up/shadow_update.
type ShadowUpdateReq struct {
	Version int64          `json:"version,omitzero"` // 0 = no optimistic lock
	State   map[string]any `json:"state"`
}

// ShadowUpdateReply is the payload for tio/{thingId}/down/shadow_update_reply.
type ShadowUpdateReply struct {
	Code    int    `json:"code"`
	Version int64  `json:"version,omitzero"`
	Message string `json:"message,omitzero"`
}

// ShadowDesired is the payload for tio/{thingId}/down/shadow_desired.
type ShadowDesired struct {
	Version int64  `json:"version"`
	Name    string `json:"name"`
	Value   any    `json:"value"`
}

// MethodReq is the payload for tio/{thingId}/down/method_req.
type MethodReq struct {
	ID     string `json:"id"`
	Method string `json:"method"`
	Data   any    `json:"data,omitzero"`
}

// MethodResp is the payload for tio/{thingId}/up/method_resp.
type MethodResp struct {
	ID      string `json:"id"`
	Code    int    `json:"code"`
	Message string `json:"message,omitzero"`
	Data    any    `json:"data,omitzero"`
}

// NtpReq is the payload for tio/{thingId}/up/ntp_req.
type NtpReq struct {
	ClientSendTime int64 `json:"clientSendTime"`
}

// NtpResp is the payload for tio/{thingId}/down/ntp_resp.
type NtpResp struct {
	Code           int    `json:"code"`
	Message        string `json:"message,omitzero"`
	ClientSendTime int64  `json:"clientSendTime,omitzero"`
	ServerRecvTime int64  `json:"serverRecvTime,omitzero"`
	ServerSendTime int64  `json:"serverSendTime,omitzero"`
}

// SimpleInvokeResult carries the device method response code and data.
type SimpleInvokeResult struct {
	Code    int
	Data    any
	Message string
}
