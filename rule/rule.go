// Package rule implements data integration rule.
// A rule is a process of data processing:
//
//	Sources --> Process(filter and transform) --> Sinks
//
// Sources an Sinks may use data Connector to get data or send data.
// Rules are assembled by Connectors, Sources and Sinks.
package rule

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	"ruff.io/tio/rule/model"
	"ruff.io/tio/rule/process"
	"ruff.io/tio/rule/sink"
	"ruff.io/tio/rule/source"
	"ruff.io/tio/shadow"
)

const (
	MsgKeyThingId = "thingId"
	MsgKeyTopic   = "topic"
	MsgKeyPayload = "payload"
	MsgKeyShadow  = "shadow"
)

type Rule interface {
	Name() string
	Start(ctx context.Context)
	Stop()
	Status() model.StatusInfo
}

type StatusGetter interface {
	Name() string
	Status() model.StatusInfo
}

type Metric struct {
	Received int64 `json:"received"`
	Passed   int64 `json:"passed"`
	Filtered int64 `json:"filtered"`
	Failed   int64 `json:"failed"`
}

func NewRule(name string,
	sources []source.Source,
	processors []process.Process,
	sinks []sink.Sink,
	shadowGetter shadow.CacheGetter,
) Rule {
	r := &ruleImpl{
		name:         name,
		sources:      sources,
		processors:   processors,
		sinks:        sinks,
		shadowGetter: shadowGetter,
	}

	runtime.SetFinalizer(r, func(obj Rule) {
		slog.Debug("Rule is being garbage collected", "type", "rule", "name", obj.Name())
	})
	return r
}

type ruleImpl struct {
	ctx       context.Context
	ctxCancel context.CancelFunc

	name         string
	shadowGetter shadow.CacheGetter
	sources      []source.Source
	processors   []process.Process
	sinks        []sink.Sink

	mu      sync.RWMutex
	started bool
	metric  Metric
}

func (r *ruleImpl) Name() string {
	return r.name
}

func (r *ruleImpl) Start(ctx context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.started {
		return
	}
	r.started = true

	r.ctx, r.ctxCancel = context.WithCancel(ctx)

	for _, src := range r.sources {
		q := make(chan source.Msg, 10000)
		src.OnMsg(r.name, func(msg source.Msg) {
			q <- msg
		})

		go r.worker(q)
	}
}

func (r *ruleImpl) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.started = false
	if r.ctxCancel != nil {
		r.ctxCancel()
	}
}

func (r *ruleImpl) Status() model.StatusInfo {
	r.mu.Lock()
	defer r.mu.Unlock()

	st := "running"
	if !r.started {
		st = "stopped"
	}
	s := model.StatusInfo{
		Status: st,
		Metric: Metric{
			Received: atomic.LoadInt64(&r.metric.Received),
			Filtered: atomic.LoadInt64(&r.metric.Filtered),
			Passed:   atomic.LoadInt64(&r.metric.Passed),
			Failed:   atomic.LoadInt64(&r.metric.Failed),
		},
	}
	return s
}

func (r *ruleImpl) worker(msgQ chan source.Msg) {
	for {
		var msg source.Msg
		select {
		case <-r.ctx.Done():
			slog.Debug("Rule worker exit cause context done", "rule", r.name)
			return
		case msg = <-msgQ:
		}
		atomic.AddInt64(&r.metric.Received, 1)

		var out string
		// process
		if pout, ok, err := r.process(msg); ok && pout != nil && err == nil {
			out = *pout
			atomic.AddInt64(&r.metric.Passed, 1)
		} else {
			if err != nil {
				atomic.AddInt64(&r.metric.Failed, 1)
			} else {
				atomic.AddInt64(&r.metric.Filtered, 1)
			}
			continue
		}

		// publish to sinks
		for _, sk := range r.sinks {
			func() {
				msg := sink.Msg{
					ThingId: msg.ThingId,
					Topic:   msg.Topic,
					Payload: string(out),
				}
				defer func() {
					if err := recover(); err != nil {
						slog.Error("Rule publish to sink", "sink", sk.Name(), "msg", msg, "error", err)
					}
				}()
				sk.Publish(msg)
			}()
		}
	}
}

func (r *ruleImpl) process(msg source.Msg) (output *string, next bool, err error) {
	defer func() {
		if e := recover(); e != nil {
			slog.Error("Rule process", "error", e, "msg", msg) // 打印错误信息
			if er, ok := e.(error); ok {
				err = er
			} else {
				err = fmt.Errorf("%v", e)
			}
		}
	}()

	output = &msg.Payload
	next = false

	if len(r.processors) == 0 {
		next = true
		return
	}

	sd, ok := r.shadowGetter.GetFromCache(msg.ThingId)
	if !ok {
		slog.Error("Rule failed to get shadow", "thingId", msg.ThingId)
	}

	input, er := MsgToProcessInput(msg, sd)
	err = er
	if err != nil {
		slog.Error("Rule failed to parse msg", "msg", msg, "error", err)
		return
	}
	hasTrans := false

	for _, p := range r.processors {
		switch p.Type() {
		case process.TypeFilter:
			o, er := p.Run(input)
			err = er
			if err != nil {
				slog.Error("Rule failed to process filter msg", "process", p.Name(), "msg", msg, "error", err)
				return
			}
			if o == true {
				continue
			} else {
				return
			}
		case process.TypeTrans:
			o, er := p.Run(input)
			err = er
			if err != nil {
				slog.Error("Rule failed to process transform msg", "process", p.Name(), "msg", msg, "error", err)
				return
			}
			input = o
			hasTrans = true
		default:
			slog.Error("Rule failed to process msg cause unknown process type", "process", p.Name(), "type", p.Type())
		}
	}

	// if has been tranformed, marshal it to string
	// otherwise use the original payload
	if hasTrans {
		b, er := marshal(input)
		err = er
		if err != nil {
			slog.Error("Rule failed to marshal process output", "msg", msg, "output", input, "error", err)
			return
		}
		output = b
	}

	next = true
	return
}

func marshal(input any) (output *string, err error) {
	if input == nil {
		return nil, nil
	}
	if s, ok := input.(string); ok {
		output = &s
	} else if arr, ok := input.([]any); ok {
		res := ""
		for _, i := range arr {
			if s, ok := i.(string); ok {
				res += s + "\n"
			} else {
				if b, err := json.Marshal(i); err == nil {
					res += string(b) + "\n"
				} else {
					return nil, fmt.Errorf("marshal %v", i)
				}
			}
		}
		res = strings.TrimSuffix(res, "\n")
		output = &res
	} else {
		b, er := json.Marshal(input)
		if er != nil {
			err = er
			return
		}
		s := string(b)
		output = &s
	}
	return
}

func MsgToProcessInput(msg source.Msg, sd shadow.ShadowWithStatus) (any, error) {
	var payload any
	err := json.Unmarshal([]byte(msg.Payload), &payload)
	if err != nil {
		return nil, err
	}

	b, err := json.Marshal(sd)
	if err != nil {
		return nil, err
	}
	var shadowVal any
	err = json.Unmarshal(b, &shadowVal)
	if err != nil {
		return nil, err
	}
	input := map[string]any{
		MsgKeyThingId: msg.ThingId,
		MsgKeyTopic:   msg.Topic,
		MsgKeyPayload: payload,
		MsgKeyShadow:  shadowVal,
	}
	return input, nil
}
