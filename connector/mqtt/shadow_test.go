package mqtt_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	mockmq "ruff.io/tio/connector/mqtt/mock"
	"ruff.io/tio/pkg/log"

	"github.com/kpango/glg"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mq "ruff.io/tio/connector/mqtt"
	"ruff.io/tio/shadow"
)

var (
	ctx = context.Background()
)

func TestHandler_GetReq(t *testing.T) {
	t.Parallel()

	thingId := fmt.Sprintf("thing-%d", time.Now().UnixNano())
	topic := strings.Replace(shadow.TopicAllGet(), "+", thingId, -1)
	mockMqtt := mockMqtt(thingId, shadow.TopicAllGet(), topic, nil, nil)

	// start handler
	h := mq.NewShadowHandler(mockMqtt)
	ch, err := h.ShadowGetReq(ctx)
	require.NoError(t, err)

	// mock thing to publish a request
	getReq := shadow.GetReq{ClientToken: fmt.Sprintf("tk-%d", time.Now().UnixNano())}
	getReqJson, _ := json.Marshal(getReq)
	mockMqtt.Publish(topic, mq.DefaultQos, false, getReqJson)

	select {
	case msg := <-ch:
		require.Equal(t, getReq.ClientToken, msg.Req.ClientToken)
		require.Equal(t, thingId, msg.ThingId)
	case <-time.After(time.Millisecond * 100):
		t.Errorf("should have response for get request")
	}

	mockMqtt.AssertExpectations(t)
}

func TestHandler_StateReq(t *testing.T) {
	t.Parallel()

	// mock
	thingId := fmt.Sprintf("thing-%d", time.Now().UnixNano())
	reqUpdateTopic := shadow.TopicUpdateOf(thingId)
	mockMqtt := mockMqtt(thingId, shadow.TopicAllUpdate(), reqUpdateTopic, nil, nil)

	// start handler

	h := mq.NewShadowHandler(mockMqtt)
	ch, err := h.StateUpdateReq(ctx)
	require.NoError(t, err)

	// mock thing to publish request

	r := shadow.StateReq{Version: 222,
		ClientToken: fmt.Sprintf("tk-%d", time.Now().UnixNano()),
		State:       shadow.StateDR{Desired: shadow.StateValue{"color": "red"}},
	}
	reqJson, _ := json.Marshal(r)
	mockMqtt.Publish(reqUpdateTopic, mq.DefaultQos, false, reqJson)

	mockMqtt.AssertExpectations(t)

	// check result

	select {
	case msg := <-ch:
		require.Equal(t, r.ClientToken, msg.Req.ClientToken)
		require.Equal(t, thingId, msg.ThingId)
		require.Equal(t, r, msg.Req)
	case <-time.After(time.Millisecond * 100):
		t.Errorf("should have response for get request")
		t.FailNow()
	}
}

func TestHandler_Accepted(t *testing.T) {
	t.Parallel()

	latestPub := struct {
		topic string
		resp  shadow.StateAcceptedResp
	}{}
	// mock mqtt client
	pubCallback := func(topic string, qos byte, retained bool, payload interface{}) {
		log.Debugf("====PUB==== topic=%q payload=%q", topic, payload)
		latestPub.topic = topic
		err := json.Unmarshal(payload.([]byte), &latestPub.resp)
		require.NoError(t, err)
	}
	mockMqtt := mockMqtt("", "", "", nil, pubCallback)

	thingId := fmt.Sprintf("thing-%d", time.Now().UnixNano())

	cases := []struct {
		topic string
		msg   shadow.StateAcceptedRespMsg
	}{
		{
			topic: shadow.TopicUpdateAcceptedOf(thingId),
			msg: shadow.StateAcceptedRespMsg{ThingId: thingId, Op: shadow.OpUpdate, Resp: shadow.StateAcceptedResp{
				Version:     3232,
				ClientToken: fmt.Sprintf("tk-%d", time.Now().UnixNano()),
				State: shadow.StateDRD{
					Desired: shadow.StateValue{"color": "green"},
				},
			}},
		},
		{
			topic: shadow.TopicGetAcceptedOf(thingId),
			msg: shadow.StateAcceptedRespMsg{ThingId: thingId, Op: shadow.OpGet, Resp: shadow.StateAcceptedResp{
				Version:     3244,
				ClientToken: fmt.Sprintf("tk-%d", time.Now().UnixNano()),
				State: shadow.StateDRD{
					Desired: shadow.StateValue{"color": "red"},
				},
			}},
		},
	}

	for _, c := range cases {
		call := mockMqtt.On("Publish", c.topic, mq.DefaultQos, false, mock.Anything).Return(mockmq.NewMockToken())
		h := mq.NewShadowHandler(mockMqtt)

		err := h.AcceptedResp(ctx, c.msg)
		require.NoError(t, err)
		mockMqtt.AssertExpectations(t)

		require.Equal(t, c.topic, latestPub.topic)
		require.Equal(t, c.msg.Resp, latestPub.resp)

		call.Unset()
	}
}

func mockMqtt(thingId, subTopic, pubTopic string, sc mockmq.SubCallback, pc mockmq.PubCallback) *mockmq.MockedMqttClient {
	cl := mockmq.NewMqttClient("", pc, sc)
	if subTopic != "" {
		cl.On("Subscribe", ctx, subTopic, mq.DefaultQos, mock.Anything).Return(nil)
	}

	if pubTopic != "" {
		token := &mockmq.Token{DoneCh: make(chan struct{})}
		close(token.DoneCh)
		cl.On("Publish", pubTopic, mq.DefaultQos, false, mock.Anything).Return(token)
	}

	return cl
}

// benchmark

func Benchmark_GetReq(b *testing.B) {
	glg.Get().SetLevel(glg.ERR)

	mqCl := mockmq.NewMqttClient("", nil, nil)
	mqCl.On("Subscribe", ctx, shadow.TopicAllGet(), mq.DefaultQos, mock.Anything).Return(nil)
	token := &mockmq.Token{DoneCh: make(chan struct{})}
	close(token.DoneCh)
	mqCl.On("Publish", mock.Anything, mq.DefaultQos, false, mock.Anything).Return(token)

	// start handler
	h := mq.NewShadowHandler(mqCl)
	ch, _ := h.ShadowGetReq(ctx)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		thingId := fmt.Sprintf("thing-%d", time.Now().UnixNano())
		topic := strings.Replace(shadow.TopicAllGet(), "+", thingId, -1)

		// mock thing to publish a request
		getReq := shadow.GetReq{ClientToken: fmt.Sprintf("tk-%d", time.Now().UnixNano())}
		getReqJson, _ := json.Marshal(getReq)

		mqCl.Publish(topic, mq.DefaultQos, false, getReqJson)
		<-ch
	}
}
