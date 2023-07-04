package mqtt_test

import (
	"encoding/json"
	"testing"
	"time"

	mockmq "ruff.io/tio/connector/mqtt/mock"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mq "ruff.io/tio/connector/mqtt"
	"ruff.io/tio/pkg/log"
	"ruff.io/tio/pkg/model"
	"ruff.io/tio/shadow"
)

func TestDirectMethodHandler_Invoke(t *testing.T) {
	t.Parallel()

	// mock mqtt client
	mockMqtt := mockmq.NewMqttClient("", nil, nil)

	// mock subscribe
	mockMqtt.On("Subscribe", mock.Anything, shadow.TopicMethodAllResponse(), mq.DefaultQos, mock.Anything).Return(nil)
	mockMqtt.On("Subscribe", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

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

	d := make(chan struct{})
	close(d)
	token := &mockmq.Token{DoneCh: d}
	for _, c := range cases {
		mockAdapter := mockmq.AdapterImpl{}
		presenceCh := make(chan shadow.Event)
		var outCh <-chan shadow.Event = presenceCh
		mockAdapter.On("OnConnect").Return(outCh)

		reqTopic := shadow.TopicMethodRequest(c.req.ThingId, c.req.Method)
		respTopic := shadow.TopicMethodResponse(c.req.ThingId, c.req.Method)

		pubCall := mockMqtt.On("Publish", reqTopic, mock.Anything, false, mock.Anything).Return(token)
		pubRespCall := mockMqtt.On("Publish", respTopic, mock.Anything, false, mock.Anything).Return(token)
		// var onlineCall *mock.Call
		var xxCall *mock.Call
		if c.req.ConnTimeout > 0 {
			// onlineCall = mockMqtt.On("IsConnected", c.req.ThingId).Return(false, nil)
			xxCall = mockAdapter.On("IsConnected", c.req.ThingId).Return(false, nil)
		} else {
			// onlineCall = mockMqtt.On("IsConnected", c.req.ThingId).Return(true, nil)
			xxCall = mockAdapter.On("IsConnected", c.req.ThingId).Return(true, nil)
		}

		handler := mq.NewMethodHandler(mockMqtt, &mockAdapter)
		err := handler.InitMethodHandler(ctx)
		require.NoError(t, err)

		// mock thing return method response
		go func() {
			respJson, _ := json.Marshal(c.resp)
			cCopy := c
			// mock thing is online
			if cCopy.req.ConnTimeout > 0 && cCopy.err == nil {
				// wait for connect
				time.Sleep(time.Millisecond * time.Duration(cCopy.req.ConnTimeout*100))
				presenceCh <- shadow.Event{ThingId: cCopy.req.ThingId, EventType: shadow.EventConnected}
			}
			// wait for method invoking
			time.Sleep(time.Millisecond * time.Duration(cCopy.timeoutMs))
			mockMqtt.Publish(respTopic, 0, false, respJson)
			log.Infof("Send mock method response")
			pubRespCall.Unset()
		}()

		// check response
		resp, err := handler.InvokeMethod(ctx, c.req)
		if c.err != nil {
			require.Truef(t, errors.Is(err, c.err), "should throw error %v", c.err)
		} else {
			require.NoError(t, err, "should no error for %s", c.req.ThingId)
			require.Equal(t, c.resp, resp, "response should be")
		}

		pubCall.Unset()
		// onlineCall.Unset()
		xxCall.Unset()
	}
}
