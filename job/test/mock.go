package test

import (
	"context"

	"github.com/stretchr/testify/mock"
	"ruff.io/tio/job"
	"ruff.io/tio/shadow"
)

func NewMockJobCenter() *JobCenter {
	return &JobCenter{}
}

type JobCenter struct {
	mock.Mock
}

var _ job.Center = &JobCenter{}

func (c *JobCenter) NotifyMgrMsg(ctx context.Context, j job.Detail) error {
	args := c.Called(ctx, j)
	if args.Get(0) == nil {
		return nil
	} else {
		return args.Get(0).(error)
	}
}

func (c *JobCenter) Start(ctx context.Context) error {
	return nil
}

func (c *JobCenter) ReceiveMgrMsg(msg job.MgrMsg) {
}

// --------------------------------mock method handler--------------------------------

type MethodHandler struct {
	mock.Mock

	returnFc func() (shadow.MethodResp, error)
}

func NewMethodHandler() MethodHandler {
	return MethodHandler{}
}

var _ shadow.MethodHandler = (*MethodHandler)(nil)

func (m *MethodHandler) InvokeMethod(ctx context.Context, req shadow.MethodReqMsg) (shadow.MethodResp, error) {
	args := m.Called(ctx, req)
	if m.returnFc != nil {
		return m.returnFc()
	}
	res := args.Get(0).(shadow.MethodResp)
	var errRes error
	if args.Get(1) != nil {
		errRes = args.Get(1).(error)
	}
	return res, errRes
}

func (m *MethodHandler) InitMethodHandler(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}

func (m *MethodHandler) SetReturnFunc(f func() (shadow.MethodResp, error)) {
	m.returnFc = f
}
