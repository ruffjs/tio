package mock

import (
	"context"
	"ruff.io/tio/connector"

	"github.com/stretchr/testify/mock"
	"ruff.io/tio/pkg/log"
)

type Connectivity struct {
	mock.Mock
}

func (g *Connectivity) AllClientInfo() ([]connector.ClientInfo, error) {
	//TODO implement me
	panic("implement me")
}

func NewConnectivity() *Connectivity {
	return &Connectivity{}
}

func (g *Connectivity) OnConnect() <-chan connector.Event {
	panic("implement me")
}

func (g *Connectivity) Start(ctx context.Context) error {
	return nil
}

func (g *Connectivity) IsConnected(thingId string) (bool, error) {
	return true, nil
}

func (g *Connectivity) ClientInfo(thingId string) (connector.ClientInfo, error) {
	return connector.ClientInfo{ClientId: thingId}, nil
}

func (g *Connectivity) Close(thingId string) error {
	log.Infof("Closed mqtt client: clientId=%q", thingId)
	args := g.Called(thingId)
	if args.Get(0) == nil {
		return nil
	} else {
		return args.Get(0).(error)
	}
}

func (g *Connectivity) Remove(thingId string) error {
	log.Infof("Remove mqtt client: clientId=%q", thingId)
	err := g.Close(thingId)
	if err != nil {
		return err
	}

	args := g.Called(thingId)
	if args.Get(0) == nil {
		return nil
	} else {
		return args.Get(0).(error)
	}
}

var _ connector.Connectivity = (*Connectivity)(nil)
