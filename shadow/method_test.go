package shadow_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/pkg/errors"
	"ruff.io/tio/connector"
	"ruff.io/tio/connector/mock"
	"ruff.io/tio/pkg/model"

	"github.com/stretchr/testify/require"
	"ruff.io/tio/shadow"
)

func TestTopicMethodRequest(t *testing.T) {
	cases := []struct {
		thingId string
		method  string
		expect  string
	}{
		{thingId: "abcd", method: "mmm1", expect: "$iothub/things/abcd/methods/mmm1/req"},
		{thingId: "xxqk", method: "m0sd5", expect: "$iothub/things/xxqk/methods/m0sd5/req"},
	}

	for _, c := range cases {
		topic := shadow.TopicMethodRequest(c.thingId, c.method)
		require.Equal(t, topic, c.expect)
	}
}

func TestTopicMethodAllResponse(t *testing.T) {
	expect := "$iothub/things/+/methods/+/resp"
	topic := shadow.TopicMethodAllResponse()
	require.Equal(t, expect, topic)
}

func TestDirectMethodHandler_Invoke(t *testing.T) {
	t.Parallel()

	cases := []struct {
		timeoutMs int
		req       shadow.MethodReqMsg
		resp      shadow.MethodResp
		err       error
	}{
		{
			timeoutMs: 50,
			req: shadow.MethodReqMsg{
				ThingId: "111111", Method: "mmmm1", RespTimeout: 1,
				Req: shadow.MethodReq{
					ClientToken: "111111",
					Data: struct {
						Color string `json:"color"`
					}{Color: "red"},
				},
			},
			resp: shadow.MethodResp{
				ClientToken: "111111",
				Data:        "xkl",
				Code:        200,
				Message:     "OK",
			},
			err: nil,
		},
		{
			timeoutMs: 1001,
			req: shadow.MethodReqMsg{
				ThingId: "222222", Method: "mmmm1", RespTimeout: 1,
				Req: shadow.MethodReq{
					ClientToken: "222222",
					Data: struct {
						Conf string `json:"conf"`
					}{Conf: "xxkkk"},
				},
			},
			err: model.ErrDirectMethodTimeout,
		},
		{
			timeoutMs: 1001,
			req: shadow.MethodReqMsg{
				ThingId: "3333333", Method: "mmmm1",
				ConnTimeout: 1,
				RespTimeout: 1,
				Req: shadow.MethodReq{
					ClientToken: "3333333",
					Data: struct {
						Color string `json:"color"`
					}{Color: "red"},
				},
			},
			err: model.ErrDirectMethodTimeout,
		},
		{
			timeoutMs: 500,
			req: shadow.MethodReqMsg{
				ThingId: "444444", Method: "mmmm1",
				ConnTimeout: 1,
				RespTimeout: 1,
				Req: shadow.MethodReq{
					ClientToken: "444444",
					Data: struct {
						Color string `json:"color"`
					}{Color: "red"},
				},
			},
			resp: shadow.MethodResp{
				ClientToken: "444444",
				Data:        "xkl",
				Code:        200,
				Message:     "OK",
			},
			err: nil,
		},
	}

	for _, c := range cases {
		mc := mock.NewMockConnector()
		_ = mc.Start(ctx)

		if c.req.ConnTimeout > 0 {
			mc.SetConnected(c.req.ThingId, false)
		} else {
			mc.SetConnected(c.req.ThingId, true)
		}

		handler := shadow.NewMethodHandler(mc)
		err := handler.InitMethodHandler(ctx)
		require.NoError(t, err)

		go func() {
			respJson, _ := json.Marshal(c.resp)
			cCopy := c
			if cCopy.req.ConnTimeout > 0 && cCopy.err == nil {
				time.Sleep(time.Millisecond * time.Duration(cCopy.req.ConnTimeout*100))
				mc.SimulatePresence(connector.PresenceEvent{ThingId: cCopy.req.ThingId, ClientId: cCopy.req.ThingId, EventType: connector.EventConnected})
				mc.SetConnected(cCopy.req.ThingId, true)
			}
			time.Sleep(time.Millisecond * time.Duration(cCopy.timeoutMs))
			respTopic := shadow.TopicMethodResponse(cCopy.req.ThingId, cCopy.req.Method)
			mc.SimulateMessage(respTopic, respJson)
		}()

		resp, err := handler.InvokeMethod(ctx, c.req)
		if c.err != nil {
			require.Truef(t, errors.Is(err, c.err), "should throw error %v", c.err)
		} else {
			require.NoError(t, err, "should no error for %s", c.req.ThingId)
			require.Equal(t, c.resp, resp, "response should be")
		}
	}
}
