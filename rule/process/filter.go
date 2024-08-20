package process

import (
	"context"
	"fmt"
)

// Filter messages

type filterProcess struct {
	name string
	jq   *jqRunner
	js   *jsRunner
}

func NewFilter(cfg Config) (Process, error) {
	BuildConfig(&cfg)
	if cfg.Runner == RunnerJq {
		j, err := NewJqRunner(cfg.Jq)
		if err != nil {
			return nil, err
		}
		return &filterProcess{cfg.Name, j, nil}, nil
	}
	if cfg.Runner == RunnerJs {
		j, err := NewJsRunner(cfg.Js)
		if err != nil {
			return nil, err
		}
		return &filterProcess{cfg.Name, nil, j}, nil
	}
	return nil, fmt.Errorf("filter must have jq or js")
}

func (f *filterProcess) Name() string {
	return f.name
}

func (f *filterProcess) Type() string {
	return TypeFilter
}

func (f *filterProcess) Run(v any) (any, error) {
	var o any
	var err error
	if f.jq != nil {
		o, err = f.jq.Run(context.Background(), v)
	} else {
		o, err = f.js.Run(context.Background(), v)
	}
	if err != nil {
		return false, err
	}
	if b, ok := o.(bool); ok {
		return b, nil
	} else {
		return false, fmt.Errorf("wrong result type %T for filter", o)
	}
}
