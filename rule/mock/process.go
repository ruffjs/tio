package mock

import (
	"github.com/stretchr/testify/mock"
	"ruff.io/tio/rule/process"
)

func NewProcess(cfg process.Config) *ProcessMock {
	return &ProcessMock{
		cfg: cfg,
	}
}

var _ process.Process = &ProcessMock{}

type ProcessMock struct {
	mock.Mock
	cfg process.Config
}

func (p *ProcessMock) Name() string {
	return p.cfg.Name
}

func (p *ProcessMock) Run(in any) (out any, err error) {
	res := p.Called(in)
	out = res.Get(0)
	e := res.Get(1)
	if e != nil {
		err = e.(error)
	}
	return
}

func (p *ProcessMock) Type() string {
	return p.cfg.Type
}
