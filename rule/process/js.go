package process

import (
	"context"
	"log/slog"

	"github.com/dop251/goja"
	"github.com/pkg/errors"
)

// TODO:  goroutine-safe https://github.com/dop251/goja?tab=readme-ov-file#is-it-goroutine-safe

type TransformFunc func(payload any) any

type jsRunner struct {
	runtime  *goja.Runtime
	function TransformFunc
}

func NewJsRunner(script string) (runner *jsRunner, err error) {
	vm := goja.New()
	if _, err := vm.RunString(script); err != nil {
		return nil, errors.WithMessage(err, "run script")
	}

	defer func() {
		if er := recover(); er != nil {
			slog.Error("Init js runner, no valid \"run\" function in script", "error", er, "script", script)
			if e, ok := er.(error); ok {
				err = e
			} else {
				err = errors.Errorf("%v", er)
			}
		}
	}()
	var f TransformFunc
	if err := vm.ExportTo(vm.Get("run"), &f); err != nil {
		return nil, errors.WithMessage(err, "export run function")
	}

	return &jsRunner{runtime: vm, function: f}, nil
}

func (q *jsRunner) Run(ctx context.Context, input any) (output any, err error) {
	defer func() {
		if er := recover(); er != nil {
			slog.Error("Run js error", "error", er, "input", input)
			if e, ok := er.(error); ok {
				err = e
			} else {
				err = errors.Errorf("%v", er)
			}
		}
	}()
	res := q.function(input)
	return res, nil
}
