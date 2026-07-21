package connector

import "context"

type AuthContext struct {
	ClientIdentifier string
	Username         string
	Password         string
	HasClientCert    bool
	CertCN           string
}

type AuthResult struct {
	Principal  string
	AuthMethod string
}

type AuthzFn func(authCtx AuthContext) (AuthResult, bool)

type AclFn func(clientId, username, topic string, write bool) bool

type BindingGetter interface {
	IsBoundGateway(ctx context.Context, thingId, gatewayThingId string) (bool, error)
}
