package api

import (
	"testing"

	"github.com/stretchr/testify/require"
	"ruff.io/tio/thing"
)

func TestCreateReqValidate(t *testing.T) {
	tests := []struct {
		name         string
		req          CreateReq
		wantErr      string
		wantAuthType string
		wantAuthVal  string
	}{
		{
			name:         "password auth is default",
			req:          CreateReq{ThingId: "dev-1", Password: "pw"},
			wantAuthType: thing.AuthTypePassword,
			wantAuthVal:  "pw",
		},
		{
			name:         "certificate auth allows empty password",
			req:          CreateReq{ThingId: "dev-cert", AuthType: thing.AuthTypeCertificate},
			wantAuthType: thing.AuthTypeCertificate,
		},
		{
			name:    "certificate auth rejects password",
			req:     CreateReq{ThingId: "dev-cert", AuthType: thing.AuthTypeCertificate, Password: "pw"},
			wantErr: "certificate auth thing cannot set password",
		},
		{
			name:    "invalid auth type should fail",
			req:     CreateReq{ThingId: "dev-x", AuthType: "token"},
			wantErr: "authType must be password or certificate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.validate()
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.wantAuthType, tt.req.authTypeOrDefault())
			require.Equal(t, tt.wantAuthVal, tt.req.authValue())
		})
	}
}
