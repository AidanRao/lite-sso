package dto

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"sso-server/util/requestmeta"
	"sso-server/util/useragent"
)

func Test_ToEnvironmentResponse_MapsOnlyPublicFields(t *testing.T) {
	environment := requestmeta.Environment{DeviceInfo: useragent.DeviceInfo{
		Label: "internal-label", Browser: "Chrome", BrowserVersion: "130.0.0.0",
		OS: "macOS", OSVersion: "10.15.7", DeviceType: "desktop", DeviceName: "Mac",
		DisplayName: "Chrome · macOS",
	}, IP: "2001:db8::1", Location: "中国 · 北京"}
	response := ToEnvironmentResponse(environment)
	require.Equal(t, EnvironmentResponse{
		Browser: "Chrome", BrowserVersion: "130.0.0.0", OS: "macOS", OSVersion: "10.15.7",
		DeviceType: "desktop", DeviceName: "Mac", DisplayName: "Chrome · macOS", IP: "2001:db8::1", Location: "中国 · 北京",
	}, response)
	raw, err := json.Marshal(response)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "user_agent")
	require.NotContains(t, string(raw), "internal-label")
}
