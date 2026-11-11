package thing

import (
	"time"
)

const (
	AuthTypePassword    string = "password"
	AuthTypeCertificate string = "certificate"

	// MaxBindThings is the max number of things that can be bound to a gateway.
	MaxBindThings = 100
)

func IsPasswordAuthType(authType string) bool {
	return authType == "" || authType == AuthTypePassword
}

func IsCertificateAuthType(authType string) bool {
	return authType == AuthTypeCertificate
}

type Thing struct {
	Id        string `json:"thingId"`
	Enabled   bool   `json:"enabled"`
	AuthType  string `json:"authType"`
	AuthValue string `json:"authValue,omitzero" optional:"true"`

	IsGateway      bool   `json:"isGateway"`
	GatewayThingId string `json:"gatewayThingId,omitzero" optional:"true"`

	MessageProtocol string `json:"messageProtocol"`
	PayloadEncoding string `json:"payloadEncoding"`

	UpdatedAt time.Time `json:"updatedAt"`
	CreatedAt time.Time `json:"createdAt"`
}

// ThingWithConnStatus is used for JOIN query results with connection status
type ThingWithConnStatus struct {
	Thing
	Connected      *bool      `json:"connected,omitzero"`
	ConnectedAt    *time.Time `json:"connectedAt,omitzero"`
	DisconnectedAt *time.Time `json:"disconnectedAt,omitzero"`
}

type ThingPatch struct {
	Enabled         *bool   `json:"enabled"`
	MessageProtocol *string `json:"messageProtocol"`
	PayloadEncoding *string `json:"payloadEncoding"`
}

type ThingBindReq struct {
	ThingIds []string `json:"thingIds"`
}
