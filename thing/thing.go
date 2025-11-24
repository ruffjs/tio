package thing

import (
	"time"
)

const (
	AuthTypePassword string = "password"
	AuthTypeCerts    string = "certs"

	// MaxBindThings is the max number of things that can be bound to a gateway.
	MaxBindThings = 100
)

type Thing struct {
	Id        string `json:"thingId"`
	Enabled   bool   `json:"enabled"`
	AuthType  string `json:"authType"`
	AuthValue string `json:"authValue,omitempty" optional:"true"`

	IsGateway      bool   `json:"isGateway"`
	GatewayThingId string `json:"gatewayThingId,omitempty" optional:"true"`

	UpdatedAt time.Time `json:"updatedAt"`
	CreatedAt time.Time `json:"createdAt"`
}

// ThingWithConnStatus is used for JOIN query results with connection status
type ThingWithConnStatus struct {
	Thing
	Connected      *bool      `json:"connected,omitempty"`
	ConnectedAt    *time.Time `json:"connectedAt,omitempty"`
	DisconnectedAt *time.Time `json:"disconnectedAt,omitempty"`
}

type ThingPatch struct {
	Enabled *bool `json:"enabled"`
}

type ThingBindReq struct {
	ThingIds []string `json:"thingIds"`
}
