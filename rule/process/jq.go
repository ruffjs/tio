package process

import (
	"context"

	"github.com/itchyny/gojq"
	"github.com/pkg/errors"
)

type jqRunner struct {
	code *gojq.Code
}

func NewJqRunner(query string) (*jqRunner, error) {
	q, err := gojq.Parse(query)
	if err != nil {
		return nil, errors.WithMessage(err, "gojq parse")
	}
	c, err := gojq.Compile(q)
	if err != nil {
		return nil, errors.WithMessage(err, "gojq compile")
	}
	return &jqRunner{code: c}, nil
}

func (q *jqRunner) Run(ctx context.Context, input any) (output any, err error) {
	iter := q.code.RunWithContext(ctx, input)
	res := make([]any, 0)
	for n, ok := iter.Next(); ok; n, ok = iter.Next() {
		if !ok {
			break
		}
		if e, ok := n.(error); ok {
			err = e
			return
		}
		res = append(res, n)
	}

	l := len(res)
	if l == 1 {
		output = res[0]
	} else if l == 0 {
		output = nil
	} else {
		output = res
	}
	return
}
