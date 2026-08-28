package reauth

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"sso-server/common"
	"sso-server/conf"
	"sso-server/dal/kv"
)

func TestService_IssuedTokenIsReusableAndSessionBound(t *testing.T) {
	service := NewService(&conf.Config{Passkey: conf.PasskeyConfig{ReauthTokenTTL: time.Minute}}, kv.NewMemoryStore())
	result, err := service.Issue(context.Background(), "user-1", "session-1", "credential-1")
	require.NoError(t, err)
	require.NotEmpty(t, result.Token)
	require.Equal(t, "Reauth", result.TokenType)

	first, err := service.Authorize(context.Background(), result.Token, "user-1", "session-1")
	require.NoError(t, err)
	second, err := service.Authorize(context.Background(), result.Token, "user-1", "session-1")
	require.NoError(t, err)
	require.Equal(t, first.CredentialID, second.CredentialID)

	_, err = service.Authorize(context.Background(), result.Token, "user-1", "session-2")
	require.ErrorIs(t, err, common.ErrReauthTokenInvalid)
	_, err = service.Authorize(context.Background(), result.Token, "user-2", "session-1")
	require.ErrorIs(t, err, common.ErrReauthTokenInvalid)
}

func TestService_MissingAndExpiredToken(t *testing.T) {
	service := NewService(&conf.Config{Passkey: conf.PasskeyConfig{ReauthTokenTTL: time.Millisecond}}, kv.NewMemoryStore())
	_, err := service.Authorize(context.Background(), "", "user-1", "session-1")
	require.ErrorIs(t, err, common.ErrReauthRequired)

	result, err := service.Issue(context.Background(), "user-1", "session-1", "credential-1")
	require.NoError(t, err)
	time.Sleep(5 * time.Millisecond)
	_, err = service.Authorize(context.Background(), result.Token, "user-1", "session-1")
	require.ErrorIs(t, err, common.ErrReauthTokenInvalid)
}
