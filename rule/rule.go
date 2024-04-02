// Package rule implements data integration rule.
// A rule is a process of data processing:
//
//	Sources --> Process(TODO: filter and transform) --> Sinks
//
// Sources an Sinks may use data Connector to get data or send data.
// Rules are assembled by Connectors, Sources and Sinks.
package rule

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/panjf2000/ants/v2"
	"ruff.io/tio/rule/process"
	"ruff.io/tio/rule/sink"
	"ruff.io/tio/rule/source"
)

const (
	MsgKeyThingId = "thingId"
	MsgKeyTopic   = "topic"
	MsgKeyPayload = "payload"
)

var gopool *ants.Pool

func init() {
	// goroutine pool may need 2KB*10000=20MB memory when pool is full
	p, err := ants.NewPool(10000, ants.WithNonblocking(true))
	gopool = p
	if err != nil {
		slog.Error("Failed to create pool for rule", "error", err)
		os.Exit(1)
	}
}

type Rule interface {
	Name() string
	Start(ctx context.Context) error
	Stop() error
}

func NewRule(name string,
	sources []source.Source,
	processors []process.Process,
	sinks []sink.Sink,
) Rule {
	r := &ruleImpl{
		name:       name,
		sources:    sources,
		processors: processors,
		sinks:      sinks,
	}

	return r
}

type ruleImpl struct {
	name       string
	sources    []source.Source
	processors []process.Process
	sinks      []sink.Sink
}

func (r *ruleImpl) Name() string {
	return r.name
}

func (r *ruleImpl) Start(ctx context.Context) error {
	for _, src := range r.sources {
		src.OnMsg(func(msg source.Msg) {
			// enable nonblocking with go pool
			err := gopool.Submit(func() {
				var out string
				// process
				if pout, ok := r.process(msg); ok && pout != nil {
					out = *pout
				} else {
					return
				}

				// publish to sinks
				for _, sk := range r.sinks {
					sk.Publish(sink.Msg{
						ThingId: msg.ThingId,
						Topic:   msg.Topic,
						Payload: string(out),
					})
				}
			})

			if err != nil {
				slog.Error("Rule failed to submit task to go pool", "ruleName", r.name,
					"msgThingId", msg.ThingId, "msgTopic", msg.Topic, "error", err)
			}
		})
		src.Start()
	}
	go func() {
		<-ctx.Done()
		r.Stop()
	}()
	return nil
}

func (r *ruleImpl) Stop() error {
	for _, src := range r.sources {
		src.Stop()
	}
	return nil
}

func (r *ruleImpl) process(msg source.Msg) (output *string, next bool) {
	output = &msg.Payload
	next = false

	if len(r.processors) == 0 {
		next = true
		return
	}

	input, err := msgToProcessInput(msg)
	if err != nil {
		slog.Error("Rule failed to parse msg", "msg", msg, "error", err)
		return
	}
	hasTrans := false

	for _, p := range r.processors {
		switch p.Type() {
		case process.TypeFilter:
			o, err := p.Run(input)
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
			o, err := p.Run(input)
			if err != nil {
				slog.Error("Rule failed to process transform msg", "process", p.Name(), "msg", msg, "error", err)
				return
			}
			input = o
			hasTrans = true
		default:
			slog.Error("Rule failed to process msg cause unknown process type", "process", p.Name(), "type", p.Type())
			os.Exit(1)
		}
	}

	// if has been tranformed, marshal it to string
	// otherwise use the original payload
	if hasTrans {
		b, err := marshal(input)
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

func msgToProcessInput(msg source.Msg) (any, error) {
	var payload any
	err := json.Unmarshal([]byte(msg.Payload), &payload)
	if err != nil {
		return nil, err
	}
	input := map[string]any{
		MsgKeyThingId: msg.ThingId,
		MsgKeyTopic:   msg.Topic,
		MsgKeyPayload: payload,
	}
	return input, nil
}
