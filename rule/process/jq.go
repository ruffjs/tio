package process

import (
	"context"

	"github.com/itchyny/gojq"
)

type jqRunner struct {
	code *gojq.Code
}

func NewJqRunner(query string) (*jqRunner, error) {
	q, err := gojq.Parse(query)
	if err != nil {
		return nil, err
	}
	c, err := gojq.Compile(q)
	if err != nil {
		return nil, err
	}
	return &jqRunner{code: c}, nil
}

func (q *jqRunner) Run(ctx context.Context, input any) (output any, err error) {
	iter := q.code.RunWithContext(ctx, input)
	for n, ok := iter.Next(); ok; n, ok = iter.Next() {
		if e, ok := n.(error); ok {
			err = e
			return
		}
		output = n
	}
	return
}
