package thing

import (
	"context"
	"time"

	"ruff.io/tio/pkg/model"
)

const (
	AuthTypePassword string = "password"
	AuthTypeCerts    string = "certs"
)

type Thing struct {
	Id        string    `json:"thingId"`
	Enabled   bool      `json:"enabled"`
	AuthType  string    `json:"authType"`
	AuthValue string    `json:"authValue,omitempty" optional:"true"`
	UpdatedAt time.Time `json:"updatedAt"`
	CreatedAt time.Time `json:"createdAt"`
}

type ThingUpdate struct {
	Enabled *bool `json:"enabled"`
}

type ThingWithStatus struct {
	Thing
	Connected      *bool      `json:"connected,omitempty"`
	ConnectedAt    *time.Time `json:"connectedAt,omitempty"`
	DisconnectedAt *time.Time `json:"disconnectedAt,omitempty"`
	RemoteAddr     string     `json:"remoteAddr,omitempty"`
}

type Repo interface {
	Create(ctx context.Context, th Thing) (Thing, error)
	Update(ctx context.Context, id string, tu ThingUpdate) error
	Delete(ctx context.Context, id string) error
	Query(ctx context.Context, pq PageQuery) (model.PageData[Thing], error)
	Get(ctx context.Context, id string) (*Thing, error)
	Exist(ctx context.Context, id string) (bool, error)
}
