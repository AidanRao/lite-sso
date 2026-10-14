package auth

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"

	"sso-server/dal/kv"
	"sso-server/model"
	"sso-server/util/useragent"
)

func Test_CompleteLoginWithContext_BoundsStoredDiagnostics(t *testing.T) {
	service := qrTestService(t, kv.NewMemoryStore())
	_, pair, err := service.CompleteLoginWithContext(context.Background(), "u1", "", LoginMetadata{
		DeviceID: "dev-browser", IP: "2001:0db8::1", UserAgent: "\x00" + strings.Repeat("设备", 300),
	}, AuthMethodPassword)
	require.NoError(t, err)
	var session model.UserSession
	require.NoError(t, service.db.First(&session, "id = ?", pair.SessionID).Error)
	require.Equal(t, "2001:db8::1", session.IP)
	require.LessOrEqual(t, len(session.UserAgent), useragent.MaxLength)
	require.True(t, utf8.ValidString(session.UserAgent))
	require.NotContains(t, session.UserAgent, "\x00")
}
