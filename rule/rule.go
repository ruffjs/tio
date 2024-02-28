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
	"log/slog"
	"os"

	"github.com/panjf2000/ants/v2"
	"ruff.io/tio/rule/sink"
	"ruff.io/tio/rule/source"
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
				// enable nonblocking with go pool
				err := gopool.Submit(func() {
					sk.Publish(sink.Msg{
						ThingId: msg.ThingId,
						Topic:   msg.Topic,
						Payload: msg.Payload,
					})
				})
				if err != nil {
					slog.Error("Rule failed to submit task to go pool", "ruleName", r.name,
						"msgThingId", msg.ThingId, "msgTopic", msg.Topic, "error", err)
				}
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