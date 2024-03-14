package process

import (
	"context"
)

// Transfer the message fields and format to the defined schema.

type Trans interface {
	Run(v any) (any, error)
}

type transProcess struct {
	name string
	jq   *jqRunner
}

func NewTrans(name, jq string) (Process, error) {
	j, err := NewJqRunner(jq)
	if err != nil {
		return nil, err
	}
	return &transProcess{name, j}, nil
}

func (f *transProcess) Name() string {
	return f.name
}
func (f *transProcess) Type() string {
	return TypeTrans
}

func (f *transProcess) Run(v any) (any, error) {
	o, err := f.jq.Run(context.Background(), v)
	if err != nil {
		return nil, err
	}
	return o, nil
}
