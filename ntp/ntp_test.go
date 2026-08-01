package ntp_test

import (
	"context"
	"encoding/json"
	"math"
	"math/rand"
	"testing"
	"time"

	"ruff.io/tio/connector/mock"
	"ruff.io/tio/ntp"
	"ruff.io/tio/pkg/codec"

	"github.com/stretchr/testify/require"
)

var ctx = context.Background()

func TestNtpHandler(t *testing.T) {
	t.Parallel()

	cases := []struct {
		thingId     string
		rttMs       int
		timeDeltaMs int
		negative    int
	}{
		{thingId: "aaa", rttMs: rand.Intn(200), timeDeltaMs: rand.Intn(3600), negative: rand.Intn(2)},
		{thingId: "bbb", rttMs: rand.Intn(200), timeDeltaMs: rand.Intn(3600), negative: rand.Intn(2)},
		{thingId: "ddd", rttMs: rand.Intn(200), timeDeltaMs: rand.Intn(3600), negative: rand.Intn(2)},
		{thingId: "ccc", rttMs: rand.Intn(200), timeDeltaMs: rand.Intn(3600), negative: rand.Intn(2)},
	}

	for _, c := range cases {
		mc := mock.NewMockConnector()
		_ = mc.Start(ctx)

		cc, _ := codec.New("json")
		handler := ntp.NewNtpHandler(mc, cc)
		err := handler.InitNtpHandler(ctx)
		require.NoError(t, err)

		clientNow := func() int64 {
			t := time.Now().UnixMilli()
			if c.negative == 1 {
				t -= int64(c.timeDeltaMs)
			} else {
				t += int64(c.timeDeltaMs)
			}
			return t
		}

		reqTopic := ntp.TopicReq(c.thingId)
		respTopic := ntp.TopicResp(c.thingId)

		req := ntp.Req{ClientSendTime: clientNow()}
		reqJson, _ := json.Marshal(req)

		mc.SimulateMessage(reqTopic, reqJson)

		var respPayload []byte
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			for _, pub := range mc.Published {
				if pub.Topic == respTopic {
					respPayload = pub.Payload
					break
				}
			}
			if respPayload != nil {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		require.NotNil(t, respPayload, "should have received ntp response for %s", c.thingId)

		var resp ntp.Resp
		err = json.Unmarshal(respPayload, &resp)
		require.NoError(t, err)
		require.Equal(t, req.ClientSendTime, resp.ClientSendTime)

		time.Sleep(time.Millisecond * time.Duration(c.rttMs/2))
		clientRecvTime := clientNow()
		calNow := calTime(resp.ClientSendTime, resp.ServerRecvTime, resp.ServerSendTime, clientRecvTime)
		now := time.Now().UnixMilli()
		diffNowMs := math.Abs(float64(now - calNow))
		require.Less(t, diffNowMs, 100.0,
			"the calculated time should be within 100ms from the current time.")
	}
}

func calTime(clientSendTime, serverRecvTime, serverSendTime, clientRecvTime int64) int64 {
	return (serverRecvTime + serverSendTime + clientRecvTime - clientSendTime) / 2
}
