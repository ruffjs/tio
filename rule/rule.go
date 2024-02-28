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

	"ruff.io/tio/rule/sink"
	"ruff.io/tio/rule/source"
)

type Rule interface {
	Start() error
	Stop() error
}

func NewRule(ctx context.Context, name string, sources []source.Source, sinks []sink.Sink) Rule {
	r := &ruleImpl{
		ctx:     ctx,
		name:    name,
		sources: sources,
		sinks:   sinks,
	}

	return r
}

type ruleImpl struct {
	ctx     context.Context
	name    string
	sources []source.Source
	sinks   []sink.Sink
}

func (r *ruleImpl) Start() error {
	for _, src := range r.sources {
		src.OnMsg(func(msg source.Msg) {
			for _, sk := range r.sinks {
				sk.Publish(sink.Msg{
					ThingId: msg.ThingId,
					Topic:   msg.Topic,
					Payload: msg.Payload,
				})
			}
		})
	}
	go func() {
		<-r.ctx.Done()
		r.Stop()
	}()
	return nil
}

func (r *ruleImpl) Stop() error {
	for _, src := range r.sources {
		src.OnMsg(nil)
	}
	return nil
}
