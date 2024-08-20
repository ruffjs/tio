package process

import (
	"context"
	"fmt"
)

// Transfer the message fields and format to the defined schema.

type transProcess struct {
	name string
	jq   *jqRunner
	js   *jsRunner
}

func NewTrans(cfg Config) (Process, error) {
	BuildConfig(&cfg)
	if cfg.Runner == RunnerJq {
		j, err := NewJqRunner(cfg.Jq)
		if err != nil {
			return nil, err
		}
		return &transProcess{cfg.Name, j, nil}, nil
	}
	if cfg.Runner == RunnerJs {
		j, err := NewJsRunner(cfg.Js)
		if err != nil {
			return nil, err
		}
		return &transProcess{cfg.Name, nil, j}, nil
	}
	return nil, fmt.Errorf("transform must have jq or js")
}

func (f *transProcess) Name() string {
	return f.name
}
func (f *transProcess) Type() string {
	return TypeTrans
}

func (f *transProcess) Run(v any) (any, error) {
	var o any
	var err error
	if f.jq != nil {
		o, err = f.jq.Run(context.Background(), v)
	} else {
		o, err = f.js.Run(context.Background(), v)
	}

	if err != nil {
		return nil, err
	}
	return o, nil
}
