package useragent

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

func Test_Parse_BrowserAndDeviceEnvironment(t *testing.T) {
	cases := []struct {
		name, ua, browser, version, os, deviceType, displayName, label string
	}{
		{name: "Chrome on Mac", ua: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36", browser: "Chrome", version: "130.0.0.0", os: "macOS", deviceType: "desktop", displayName: "Chrome · macOS", label: "Mac"},
		{name: "Edge before Chrome", ua: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36 Edg/130.0.2849.68", browser: "Edge", version: "130.0.2849.68", os: "Windows", deviceType: "desktop", displayName: "Edge · Windows", label: "Windows"},
		{name: "Firefox on iPhone", ua: "Mozilla/5.0 (iPhone; CPU iPhone OS 17_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) FxiOS/130.0 Mobile/15E148 Safari/605.1.15", browser: "Firefox", version: "130.0", os: "iOS", deviceType: "mobile", displayName: "Firefox · iOS", label: "iPhone"},
		{name: "Safari on iPad", ua: "Mozilla/5.0 (iPad; CPU OS 17_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.6 Mobile/15E148 Safari/604.1", browser: "Safari", version: "17.6", os: "iOS", deviceType: "tablet", displayName: "Safari · iOS", label: "iPad"},
		{name: "Chrome on Android", ua: "Mozilla/5.0 (Linux; Android 14; Pixel 8 Build/AP1A) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Mobile Safari/537.36", browser: "Chrome", version: "130.0.0.0", os: "Android", deviceType: "mobile", displayName: "Chrome · Android", label: "Android"},
		{name: "ChromeOS", ua: "Mozilla/5.0 (X11; CrOS x86_64 16033.0.0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36", browser: "Chrome", version: "130.0.0.0", os: "ChromeOS", deviceType: "desktop", displayName: "Chrome · ChromeOS", label: "ChromeOS"},
		{name: "empty", deviceType: "unknown", displayName: "未知设备"},
		{name: "unrecognized must not echo input", ua: "private-client-secret/1.0", deviceType: "unknown", displayName: "未知设备"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			info := Parse(tc.ua)
			require.Equal(t, tc.browser, info.Browser)
			require.Equal(t, tc.version, info.BrowserVersion)
			require.Equal(t, tc.os, info.OS)
			require.Equal(t, tc.deviceType, info.DeviceType)
			require.Equal(t, tc.displayName, info.DisplayName)
			require.Equal(t, tc.label, info.Label)
		})
	}
}

func Test_Parse_LabelUsesParsedDeviceInformation(t *testing.T) {
	// A product name containing a device keyword must not identify that device.
	require.Empty(t, Parse("custom-client-iphone/1.0").Label)
	require.Equal(t, "Mac", Parse("Mozilla/5.0 (Macintosh; Intel Mac OS X)").Label)
}

func Test_Normalize_BoundedValidText(t *testing.T) {
	require.Equal(t, "Chrome/130.0", Normalize(" \tChrome/130.0\r\n\x00 "))
	normalized := Normalize(strings.Repeat("设备", 300))
	require.LessOrEqual(t, len(normalized), MaxLength)
	require.True(t, utf8.ValidString(normalized))
	require.Equal(t, "Chrome", Normalize("Chrome\xff"))
}
