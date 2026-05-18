package auth_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"ruff.io/tio/auth"
	"ruff.io/tio/config"
	"ruff.io/tio/connector/mqtt/embed"
	"ruff.io/tio/pkg/model"
	"ruff.io/tio/shadow"
	"ruff.io/tio/thing"
)

func TestAuthzMqttClient(t *testing.T) {
	tests := []struct {
		name          string
		superUsers    []config.UserPassword
		namespaces    []config.Namespace
		thingSvc      *stubThingService
		provision     thing.Provision
		authCtx       embed.AuthContext
		wantOK        bool
		wantPrincipal string
		wantMethod    string
	}{
		{
			name: "password thing should authenticate with password",
			thingSvc: &stubThingService{
				things: map[string]thing.Thing{
					"dev-1": {Id: "dev-1", Enabled: true, AuthType: thing.AuthTypePassword, AuthValue: "pw-1"},
				},
			},
			authCtx: embed.AuthContext{
				ClientIdentifier: "cid-1",
				Username:         "dev-1",
				Password:         "pw-1",
				Clean:            true,
			},
			wantOK:        true,
			wantPrincipal: "dev-1",
			wantMethod:    "password",
		},
		{
			name: "certificate thing should authenticate with certificate common name",
			thingSvc: &stubThingService{
				things: map[string]thing.Thing{
					"dev-cert": {Id: "dev-cert", Enabled: true, AuthType: thing.AuthTypeCertificate},
				},
			},
			authCtx: embed.AuthContext{
				ClientIdentifier: "cid-cert",
				Username:         "dev-cert",
				Clean:            true,
				HasClientCert:    true,
				CertCN:           "dev-cert",
			},
			wantOK:        true,
			wantPrincipal: "dev-cert",
			wantMethod:    "certificate",
		},
		{
			name: "certificate thing should reject password auth",
			thingSvc: &stubThingService{
				things: map[string]thing.Thing{
					"dev-cert": {Id: "dev-cert", Enabled: true, AuthType: thing.AuthTypeCertificate},
				},
			},
			authCtx: embed.AuthContext{
				ClientIdentifier: "cid-cert",
				Username:         "dev-cert",
				Password:         "pw",
				Clean:            true,
			},
			wantOK: false,
		},
		{
			name: "certificate thing should reject mismatched username",
			thingSvc: &stubThingService{
				things: map[string]thing.Thing{
					"dev-cert": {Id: "dev-cert", Enabled: true, AuthType: thing.AuthTypeCertificate},
				},
			},
			authCtx: embed.AuthContext{
				ClientIdentifier: "cid-cert",
				Username:         "other-user",
				Clean:            true,
				HasClientCert:    true,
				CertCN:           "dev-cert",
			},
			wantOK: false,
		},
		{
			name: "password thing should reject certificate auth",
			thingSvc: &stubThingService{
				things: map[string]thing.Thing{
					"dev-1": {Id: "dev-1", Enabled: true, AuthType: thing.AuthTypePassword, AuthValue: "pw-1"},
				},
			},
			authCtx: embed.AuthContext{
				ClientIdentifier: "cid-1",
				Clean:            true,
				HasClientCert:    true,
				CertCN:           "dev-1",
			},
			wantOK: false,
		},
		{
			name: "certificate thing should reject empty common name",
			thingSvc: &stubThingService{
				things: map[string]thing.Thing{
					"dev-cert": {Id: "dev-cert", Enabled: true, AuthType: thing.AuthTypeCertificate},
				},
			},
			authCtx: embed.AuthContext{
				ClientIdentifier: "cid-cert",
				Clean:            true,
				HasClientCert:    true,
			},
			wantOK: false,
		},
		{
			name: "certificate thing should reject disabled device",
			thingSvc: &stubThingService{
				things: map[string]thing.Thing{
					"dev-cert": {Id: "dev-cert", Enabled: false, AuthType: thing.AuthTypeCertificate},
				},
			},
			authCtx: embed.AuthContext{
				ClientIdentifier: "cid-cert",
				Clean:            true,
				HasClientCert:    true,
				CertCN:           "dev-cert",
			},
			wantOK: false,
		},
		{
			name: "certificate thing should reject unknown device",
			thingSvc: &stubThingService{
				things: map[string]thing.Thing{},
			},
			authCtx: embed.AuthContext{
				ClientIdentifier: "cid-cert",
				Clean:            true,
				HasClientCert:    true,
				CertCN:           "missing-cert",
			},
			wantOK: false,
		},
		{
			name: "password thing should reject disabled device",
			thingSvc: &stubThingService{
				things: map[string]thing.Thing{
					"dev-1": {Id: "dev-1", Enabled: false, AuthType: thing.AuthTypePassword, AuthValue: "pw-1"},
				},
			},
			authCtx: embed.AuthContext{
				ClientIdentifier: "cid-1",
				Username:         "dev-1",
				Password:         "pw-1",
				Clean:            true,
			},
			wantOK: false,
		},
		{
			name: "super user should reject certificate auth",
			superUsers: []config.UserPassword{{
				Name:     "admin",
				Password: "secret",
			}},
			thingSvc: &stubThingService{things: map[string]thing.Thing{}},
			authCtx: embed.AuthContext{
				ClientIdentifier: "cid-admin",
				Clean:            true,
				HasClientCert:    true,
				CertCN:           "admin",
			},
			wantOK: false,
		},
		{
			name: "thing should reject clean session false",
			thingSvc: &stubThingService{
				things: map[string]thing.Thing{
					"dev-1": {Id: "dev-1", Enabled: true, AuthType: thing.AuthTypePassword, AuthValue: "pw-1"},
				},
			},
			authCtx: embed.AuthContext{
				ClientIdentifier: "cid-1",
				Username:         "dev-1",
				Password:         "pw-1",
				Clean:            false,
			},
			wantOK: false,
		},
		{
			name: "super user should authenticate independently",
			superUsers: []config.UserPassword{{
				Name:     "admin",
				Password: "secret",
			}},
			thingSvc: &stubThingService{things: map[string]thing.Thing{}},
			authCtx: embed.AuthContext{
				ClientIdentifier: "cid-admin",
				Username:         "admin",
				Password:         "secret",
				Clean:            true,
			},
			wantOK:        true,
			wantPrincipal: "admin",
			wantMethod:    "superuser-password",
		},
		{
			name: "should provision password thing on demand",
			thingSvc: &stubThingService{
				getErr: fmt.Errorf("missing: %w", model.ErrNotFound),
			},
			provision: stubProvision{ok: true},
			authCtx: embed.AuthContext{
				ClientIdentifier: "cid-provision",
				Username:         "new-thing",
				Password:         "Hxxx",
				Clean:            true,
			},
			wantOK:        true,
			wantPrincipal: "new-thing",
			wantMethod:    "password-provision",
		},
		{
			name: "namespace user should authenticate independently",
			namespaces: []config.Namespace{{
				Name: "biz",
				Users: []config.UserPassword{{
					Name:     "$biz",
					Password: "secret",
				}},
			}},
			thingSvc: &stubThingService{things: map[string]thing.Thing{}},
			authCtx: embed.AuthContext{
				ClientIdentifier: "cid-biz",
				Username:         "$biz",
				Password:         "secret",
				Clean:            true,
			},
			wantOK:        true,
			wantPrincipal: "$ns/biz",
			wantMethod:    "namespace-password",
		},
		{
			name: "namespace user should support clean session false",
			namespaces: []config.Namespace{{
				Name: "biz",
				Users: []config.UserPassword{{
					Name:     "$biz",
					Password: "secret",
				}},
			}},
			thingSvc: &stubThingService{things: map[string]thing.Thing{}},
			authCtx: embed.AuthContext{
				ClientIdentifier: "cid-biz-persistent",
				Username:         "$biz",
				Password:         "secret",
				Clean:            false,
			},
			wantOK:        true,
			wantPrincipal: "$ns/biz",
			wantMethod:    "namespace-password",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authz := auth.AuthzMqttClient(context.Background(), tt.superUsers, tt.thingSvc, tt.provision, tt.namespaces)

			result, ok := authz(tt.authCtx)
			require.Equal(t, tt.wantOK, ok)
			if !tt.wantOK {
				return
			}
			require.Equal(t, tt.wantPrincipal, result.Principal)
			require.Equal(t, tt.wantMethod, result.AuthMethod)
		})
	}
}

type stubThingService struct {
	things      map[string]thing.Thing
	getErr      error
	updateAuths map[string]string
}

func (s *stubThingService) Create(context.Context, thing.Thing, shadow.TagsValue, bool) (thing.Thing, error) {
	panic("unexpected call")
}

func (s *stubThingService) Update(context.Context, string, thing.ThingPatch) error {
	panic("unexpected call")
}

func (s *stubThingService) Delete(context.Context, string) error {
	panic("unexpected call")
}

func (s *stubThingService) Query(context.Context, thing.PageQuery) (thing.Page, error) {
	panic("unexpected call")
}

func (s *stubThingService) Get(_ context.Context, id string) (*thing.Thing, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	th, ok := s.things[id]
	if !ok {
		return nil, fmt.Errorf("%s: %w", id, model.ErrNotFound)
	}
	return &th, nil
}

func (s *stubThingService) Exist(context.Context, string) (bool, error) {
	panic("unexpected call")
}

func (s *stubThingService) UpdateAuthValue(_ context.Context, id string, authValue string) error {
	if s.updateAuths == nil {
		s.updateAuths = map[string]string{}
	}
	s.updateAuths[id] = authValue
	return nil
}

func (s *stubThingService) BindToGateway(context.Context, []string, string) error {
	panic("unexpected call")
}

func (s *stubThingService) UnbindFromGateway(context.Context, []string, string) error {
	panic("unexpected call")
}

func (s *stubThingService) IsBoundGateway(context.Context, string, string) (bool, error) {
	panic("unexpected call")
}

type stubProvision struct {
	ok bool
}

func (s stubProvision) AutoRegisterViaHmac(_ context.Context, thingId, password string) (bool, thing.Thing, error) {
	if !s.ok {
		return false, thing.Thing{}, nil
	}
	return true, thing.Thing{Id: thingId, AuthType: thing.AuthTypePassword, AuthValue: password, Enabled: true}, nil
}
