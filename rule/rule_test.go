package rule_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"ruff.io/tio/rule"
	innerMock "ruff.io/tio/rule/mock"
	"ruff.io/tio/rule/process"
	"ruff.io/tio/rule/sink"
	"ruff.io/tio/rule/source"
	"ruff.io/tio/shadow"
)

func Test_RuleBasic(t *testing.T) {
	src := innerMock.NewSource("mock-source")
	sk := innerMock.NewSink("mock-sink")
	shadowGetter := innerMock.NewShadowGetter()

	pfilter := innerMock.NewProcess(process.Config{
		Name: "mock-process-filter",
		Type: process.TypeFilter,
	})
	ptrans := innerMock.NewProcess(process.Config{
		Name: "mock-process-trans",
		Type: process.TypeTrans,
	})

	cases := []struct {
		name       string
		srcMsg     source.Msg
		sinkMsg    sink.Msg
		filterPass bool
	}{
		{
			name: "happy path",
			srcMsg: source.Msg{
				ThingId: "test-thing",
				Topic:   "$iothub/things/test-thing/hi",
				Payload: "{}",
			},
			sinkMsg: sink.Msg{
				ThingId: "test-thing",
				Topic:   "$iothub/things/test-thing/hi",
				Payload: "{\"test\": 1}",
			},
			filterPass: true,
		},
		{
			name: "filtered",
			srcMsg: source.Msg{
				ThingId: "test-thing",
				Topic:   "$iothub/things/test-thing/hi",
				Payload: "{}",
			},
			sinkMsg:    sink.Msg{},
			filterPass: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srcMsg := c.srcMsg
			sinkMsg := c.sinkMsg

			srcStopCall := src.On("Stop").Once()
			srcStartCall := src.On("Start").Once()
			pubCall := sk.On("Publish", mock.Anything).Once()
			ptRunCall := ptrans.On("Run", mock.Anything).Return(sinkMsg.Payload, nil).Once()
			pfRunCall := pfilter.On("Run", mock.Anything).Return(c.filterPass, nil).Once()
			if !c.filterPass {
				pubCall.Times(0)
				ptRunCall.Times(0)
			}

			shadowGetCall := shadowGetter.On("GetFromCache", mock.Anything).
				Return(shadow.ShadowWithStatus{Shadow: shadow.Shadow{Tags: shadow.TagsValue{"deviceId": 789}}}, true).Once()

			ctx, cancel := context.WithCancel(context.Background())
			r := rule.NewRule(
				"mock-rule",
				[]source.Source{src},
				[]process.Process{pfilter, ptrans},
				[]sink.Sink{sk},
				shadowGetter,
			)
			r.Start(ctx)
			src.MockMsg(srcMsg)

			// Wait process
			time.Sleep(time.Millisecond * 5)

			// Process filter and transform should have run
			pfilter.AssertExpectations(t)
			ptrans.AssertExpectations(t)

			shadowGetter.AssertExpectations(t)

			if c.filterPass {
				// Should Publish message to sink
				sk.AssertCalled(t, "Publish", sinkMsg)
			}

			// Source "Stop" method should be invoked
			cancel()
			time.Sleep(time.Millisecond)
			srcStopCall.Parent.AssertExpectations(t)

			pubCall.Unset()
			ptRunCall.Unset()
			pfRunCall.Unset()
			srcStopCall.Unset()
			srcStartCall.Unset()
			shadowGetCall.Unset()
		})
	}

}

func Test_RuleMultipleSrcMultipleSinks(t *testing.T) {
	// Multiple source messages should be published to multiple Sinks

	src1 := innerMock.NewSource("mock-source-1")
	src2 := innerMock.NewSource("mock-source-2")
	sk1 := innerMock.NewSink("mock-sink-1")
	sk2 := innerMock.NewSink("mock-sink-2")
	shadowGetter := innerMock.NewShadowGetter()

	ptrans := innerMock.NewProcess(process.Config{
		Name: "mock-process-trans",
		Type: process.TypeTrans,
	})

	srcMsg := source.Msg{
		ThingId: "test-thing",
		Topic:   "$iothub/things/test-thing/hi",
		Payload: "{}",
	}
	sinkMsg := sink.Msg{
		ThingId: "test-thing",
		Topic:   "$iothub/things/test-thing/hi",
		Payload: "{}",
	}
	srcStartCall1 := src1.On("Start")
	srcStartCall2 := src2.On("Start")
	srcStopCall1 := src1.On("Stop")
	srcStopCall2 := src2.On("Stop")
	pubCall1 := sk1.On("Publish", mock.Anything).Once()
	pubCall2 := sk2.On("Publish", mock.Anything).Once()
	ptRunCall := ptrans.On("Run", mock.Anything).Return(sinkMsg.Payload, nil).Once()

	shadowGetCall := shadowGetter.On("GetFromCache", mock.Anything).
		Return(shadow.ShadowWithStatus{}, true).Once()

	ctx, cancel := context.WithCancel(context.Background())
	r := rule.NewRule(
		"mock-rule",
		[]source.Source{src1, src2},
		[]process.Process{ptrans},
		[]sink.Sink{sk1, sk2},
		shadowGetter,
	)
	r.Start(ctx)

	src1.MockMsg(srcMsg)
	src2.MockMsg(srcMsg)

	// Wait process
	time.Sleep(time.Millisecond)

	// Process  should have run
	ptrans.AssertExpectations(t)

	// Should Publish message to sink
	sk1.AssertCalled(t, "Publish", sinkMsg)
	sk2.AssertCalled(t, "Publish", sinkMsg)

	// Source "Start" and "Stop" method should be invoked
	cancel()
	time.Sleep(time.Millisecond)
	srcStopCall1.Parent.AssertExpectations(t)
	srcStopCall2.Parent.AssertExpectations(t)

	pubCall1.Unset()
	pubCall2.Unset()
	ptRunCall.Unset()
	srcStartCall1.Unset()
	srcStartCall2.Unset()
	srcStopCall1.Unset()
	srcStopCall2.Unset()
	shadowGetCall.Unset()
}

func Test_RuleProcessMarshal(t *testing.T) {
	cases := []struct {
		input  any
		output any
		hasErr bool
	}{
		{
			input:  "hi",
			output: "hi",
		},
		{
			input:  []any{"a", "b"},
			output: "a\nb",
		},
		{
			input:  map[string]any{"a": 1, "x": "y"},
			output: "{\"a\":1,\"x\":\"y\"}",
		},
		{
			input:  nil,
			output: nil,
		},
	}

	for _, c := range cases {
		o, err := rule.Marshal_for_test(c.input)
		if c.hasErr {
			require.Errorf(t, err, "input %v should has error", c.input)
			continue
		}
		var out any = nil
		if o != nil {
			out = *o
		}
		require.Equal(t, c.output, out)
	}
}
