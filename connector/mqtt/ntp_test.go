package mqtt_test

import (
	"encoding/json"
	"math"
	"math/rand"
	"testing"
	"time"

	mockmq "ruff.io/tio/connector/mqtt/mock"
	"ruff.io/tio/ntp"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mq "ruff.io/tio/connector/mqtt"
	"ruff.io/tio/pkg/log"
)

func TestNtpHandler(t *testing.T) {
	t.Parallel()

	// mock mqtt client
	mockMqtt := mockmq.NewMqttClient("", nil, nil)

	// mock subscribe
	mockMqtt.On("Subscribe", mock.Anything, ntp.TopicReqAll, mq.DefaultQos, mock.Anything).Return(nil)
	mockMqtt.On("Subscribe", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	cases := []struct {
		thingId string
		// 网络一个 RTT 的二分之一时间
		rttMs       int
		timeDeltaMs int
		negative    int
	}{
		{
			thingId:     "aaa",
			rttMs:       rand.Intn(200),
			timeDeltaMs: rand.Intn(3600),
			negative:    rand.Intn(2),
		},
		{
			thingId:     "bbb",
			rttMs:       rand.Intn(200),
			timeDeltaMs: rand.Intn(3600),
			negative:    rand.Intn(2),
		},
		{
			thingId:     "ddd",
			rttMs:       rand.Intn(200),
			timeDeltaMs: rand.Intn(3600),
			negative:    rand.Intn(2),
		},
		{
			thingId:     "ccc",
			rttMs:       rand.Intn(200),
			timeDeltaMs: rand.Intn(3600),
			negative:    rand.Intn(2),
		},
	}

	d := make(chan struct{})
	close(d)
	token := &mockmq.Token{DoneCh: d}
	for _, c := range cases {

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

		pubReq := mockMqtt.On("Publish", reqTopic, mock.Anything, false, mock.Anything).Return(token)
		pubResp := mockMqtt.On("Publish", respTopic, mock.Anything, false, mock.Anything).Return(token)

		handler := mq.NewNtpHandler(mockMqtt)
		err := handler.InitNtpHandler(ctx)
		require.NoError(t, err)

		req := ntp.Req{ClientSendTime: clientNow()}

		// subcribe response
		respCh := make(chan mqtt.Message, 1)
		err = mockMqtt.Subscribe(ctx, respTopic, 1, func(cl mqtt.Client, m mqtt.Message) {
			respCh <- m
		})
		require.NoError(t, err)

		// request
		go func() {
			reqJson, _ := json.Marshal(req)
			// mock request time
			time.Sleep(time.Millisecond * time.Duration(c.rttMs/2))
			mockMqtt.Publish(reqTopic, 0, false, reqJson)
			log.Infof("Send mock ntp request")
			pubReq.Unset()
		}()

		select {
		case <-time.After(time.Second * 2):
			require.True(t, false, "timeout")
		case m := <-respCh:
			// mock response time, assume requestTime == responseTime
			time.Sleep(time.Millisecond * time.Duration(c.rttMs/2))
			var resp ntp.Resp
			err := json.Unmarshal(m.Payload(), &resp)
			require.NoError(t, err)
			require.Equal(t, req.ClientSendTime, resp.ClientSendTime,
				"clientSendTime in req and resp should be equal")
			clientRecvTime := clientNow()
			calNow := calTime(resp.ClientSendTime, resp.ServerRecvTime, resp.ServerSendTime, clientRecvTime)
			now := time.Now().UnixMilli()
			diffNowMs := math.Abs(float64(now - calNow))
			require.Less(t, diffNowMs, 10.0,
				"The calculated time should be within 1ms from the current time.")
		}

		pubResp.Unset()
	}
}

func calTime(clientSendTime, serverRecvTime, serverSendTime, clientRecvTime int64) int64 {
	return (serverRecvTime + serverSendTime + clientRecvTime - clientSendTime) / 2
}
