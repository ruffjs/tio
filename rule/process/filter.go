package process

import (
	"context"
	"fmt"
)

// Filter messages

type filterProcess struct {
	name string
	jq   *jqRunner
}

func NewFilter(name, jq string) (Process, error) {
	j, err := NewJqRunner(jq)
	if err != nil {
		return nil, err
	}
	return &filterProcess{name, j}, nil
}

func (f *filterProcess) Name() string {
	return f.name
}

func (f *filterProcess) Type() string {
	return TypeFilter
}

func (f *filterProcess) Run(v any) (any, error) {
	o, err := f.jq.Run(context.Background(), v)
	if err != nil {
		return false, err
	}
	if b, ok := o.(bool); ok {
		return b, nil
	} else {
		return false, fmt.Errorf("wrong result type %T for filter", o)
	}
}
