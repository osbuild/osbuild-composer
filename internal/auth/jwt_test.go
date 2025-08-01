package auth_test

import (
	"context"
	"testing"

	"github.com/golang-jwt/jwt/v4"
	"github.com/openshift-online/ocm-sdk-go/authentication"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/osbuild-composer/internal/auth"
)

func TestChannelFromContext(t *testing.T) {
	// tokens generated by https://jwt.io/ and signed by "osbuild"
	tests := []struct {
		name           string
		token          string
		value          string
		expectedFields []string
		err            error
	}{
		{
			name:           "rh-org-id=42",
			token:          "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJyaC1vcmctaWQiOiI0MiJ9.D5EwgcrlCPcPamM9hL63bWI7xxr0YVWxsJ4f80toQv4",
			value:          "42",
			expectedFields: []string{"rh-org-id"},
			err:            nil,
		},
		{
			name:           "no rh-org-id",
			token:          "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.e30.AmoXfoVMgoq4H-XsD7lTGgY6QJCW1914aYlmGnj7wtY",
			value:          "",
			expectedFields: []string{"rh-org-id"},
			err:            auth.ErrNoKey,
		},
		{
			name:           "no rh-org-id but account_id=123",
			token:          "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJhY2NvdW50X2lkIjoiMTIzIn0.fng__koaJeF3Ef6E8kFOKCMm6U2MTwyFQ4s0G4LBUss",
			value:          "123",
			expectedFields: []string{"rh-org-id", "account_id"},
			err:            nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims := jwt.MapClaims{}
			token, err := jwt.ParseWithClaims(tt.token, claims, func(token *jwt.Token) (interface{}, error) {
				return []byte("osbuild"), nil
			})
			require.NoError(t, err)

			ctx := authentication.ContextWithToken(context.Background(), token)
			channel, err := auth.GetFromClaims(ctx, tt.expectedFields)
			require.Equal(t, tt.err, err)
			require.Equal(t, tt.value, channel)

		})
	}

	t.Run("no jwt token in context", func(t *testing.T) {
		channel, err := auth.GetFromClaims(context.Background(), []string{"osbuild!"})
		require.ErrorIs(t, err, auth.ErrNoJWT)
		require.Equal(t, "", channel)
	})
}
