package shadow

import (
	"context"
	"strings"
)

const (
	TopicMethodPrefix = TopicThingsPrefix + "{thingId}/methods/{methodName}"
	TopicMethodReq    = "/req"
	TopicMethodResp   = "/resp"
)

type MethodReqMsg struct {
	ThingId     string `json:"thingId"`
	Method      string `json:"method"`
	ConnTimeout int    `json:"connTimeout"`     // seconds
	RespTimeout int    `json:"responseTimeout"` // seconds
	Req         MethodReq
}

type MethodReq struct {
	ClientToken string `json:"clientToken,omitempty"`
	Data        any    `json:"data,omitempty"`
}

type MethodResp struct {
	ClientToken string `json:"clientToken,omitempty"`
	Data        any    `json:"data,omitempty"`
	Code        int    `json:"code"`
	Message     string `json:"message"`
}

type MethodRespMsg struct {
	ThingId string `json:"thingId"`
	Resp    MethodResp
}

type MethodHandler interface {
	InvokeMethod(ctx context.Context, req MethodReqMsg) (MethodResp, error)
	InitMethodHandler(ctx context.Context) error
}

func TopicMethodRequest(thingId, methodName string) string {
	return topicMethodPrefix(thingId, methodName) + TopicMethodReq
}

func TopicMethodResponse(thingId, methodName string) string {
	return topicMethodPrefix(thingId, methodName) + TopicMethodResp
}

func TopicMethodAllResponse() string {
	s := strings.Replace(TopicMethodPrefix, "{thingId}", "+", -1) + TopicMethodResp
	s = strings.Replace(s, "{methodName}", "+", -1)
	return s
}

func topicMethodPrefix(thingId, methodName string) string {
	s := strings.Replace(TopicMethodPrefix, "{thingId}", thingId, -1)
	s = strings.Replace(s, "{methodName}", methodName, -1)
	return s
}
