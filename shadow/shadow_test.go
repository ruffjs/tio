package shadow_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"ruff.io/tio/connector/mock"
	"ruff.io/tio/pkg/codec"
	"ruff.io/tio/shadow"

	"github.com/stretchr/testify/require"
)

func TestHandler_GetReq(t *testing.T) {
	t.Parallel()

	thingId := fmt.Sprintf("thing-%d", time.Now().UnixNano())
	topic := strings.Replace(shadow.TopicAllGet(), "+", thingId, -1)
	mc := mock.NewMockConnector()
	_ = mc.Start(ctx)

	h := shadow.NewShadowHandler(mc, mustCodec(t))
	ch, err := h.ShadowGetReq(ctx)
	require.NoError(t, err)

	getReq := shadow.GetReq{ClientToken: fmt.Sprintf("tk-%d", time.Now().UnixNano())}
	getReqJson, _ := json.Marshal(getReq)
	mc.SimulateMessage(topic, getReqJson)

	select {
	case msg := <-ch:
		require.Equal(t, getReq.ClientToken, msg.Req.ClientToken)
		require.Equal(t, thingId, msg.ThingId)
	case <-time.After(time.Millisecond * 100):
		t.Errorf("should have response for get request")
	}
}

func TestHandler_StateReq(t *testing.T) {
	t.Parallel()

	thingId := fmt.Sprintf("thing-%d", time.Now().UnixNano())
	reqUpdateTopic := shadow.TopicUpdateOf(thingId)
	mc := mock.NewMockConnector()
	_ = mc.Start(ctx)

	h := shadow.NewShadowHandler(mc, mustCodec(t))
	ch, err := h.StateUpdateReq(ctx)
	require.NoError(t, err)

	r := shadow.StateReq{Version: 222,
		ClientToken: fmt.Sprintf("tk-%d", time.Now().UnixNano()),
		State:       shadow.StateDR{Desired: shadow.StateValue{"color": "red"}},
	}
	reqJson, _ := json.Marshal(r)
	mc.SimulateMessage(reqUpdateTopic, reqJson)

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
		mc := mock.NewMockConnector()
		_ = mc.Start(ctx)
		h := shadow.NewShadowHandler(mc, mustCodec(t))

		err := h.AcceptedResp(ctx, c.msg)
		require.NoError(t, err)

		require.GreaterOrEqual(t, len(mc.Published), 1)
		found := false
		for _, pub := range mc.Published {
			if pub.Topic == c.topic {
				var resp shadow.StateAcceptedResp
				err := json.Unmarshal(pub.Payload, &resp)
				require.NoError(t, err)
				require.Equal(t, c.msg.Resp, resp)
				found = true
				break
			}
		}
		require.True(t, found, "should have published to topic %s", c.topic)
	}
}

func Benchmark_GetReq(b *testing.B) {
	mc := mock.NewMockConnector()
	_ = mc.Start(ctx)

	c, _ := codec.New("json")
	h := shadow.NewShadowHandler(mc, c)
	ch, _ := h.ShadowGetReq(ctx)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		thingId := fmt.Sprintf("thing-%d", time.Now().UnixNano())
		topic := strings.Replace(shadow.TopicAllGet(), "+", thingId, -1)

		getReq := shadow.GetReq{ClientToken: fmt.Sprintf("tk-%d", time.Now().UnixNano())}
		getReqJson, _ := json.Marshal(getReq)

		mc.SimulateMessage(topic, getReqJson)
		<-ch
	}
}

func mustCodec(t *testing.T) codec.Codec {
	t.Helper()
	c, err := codec.New("json")
	require.NoError(t, err)
	return c
}
