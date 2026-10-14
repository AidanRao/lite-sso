// Package useragent extracts reusable, best-effort device information from User-Agent values.
package useragent

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/mileusna/useragent"
)

const (
	// MaxLength matches the storage limit for diagnostic User-Agent values.
	MaxLength = 512
)

// DeviceInfo contains display-oriented information inferred from a User-Agent.
// It must not be used for authentication or authorization decisions.
type DeviceInfo struct {
	Label          string
	Browser        string
	BrowserVersion string
	OS             string
	OSVersion      string
	DeviceType     string
	DeviceName     string
	DisplayName    string
}

// Normalize bounds diagnostic input and removes invalid UTF-8 and control characters.
func Normalize(value string) string {
	var result strings.Builder
	for _, char := range value {
		if char == utf8.RuneError || unicode.IsControl(char) {
			continue
		}
		if result.Len()+utf8.RuneLen(char) > MaxLength {
			break
		}
		result.WriteRune(char)
	}
	return strings.TrimSpace(result.String())
}

// Parse returns best-effort display information without echoing unrecognized clients.
// Versions are reported by the client and may be reduced; they are not verified.
func Parse(value string) DeviceInfo {
	value = Normalize(value)
	parsed := useragent.Parse(value)
	info := DeviceInfo{
		Label: deviceLabel(parsed), OS: parsed.OS, OSVersion: numericVersion(parsed.OSVersion),
		DeviceName: parsed.Device, DeviceType: "unknown",
	}
	switch parsed.Name {
	case useragent.Chrome, useragent.Firefox, useragent.Safari, useragent.Edge,
		useragent.Opera, useragent.OperaMini, useragent.OperaTouch, useragent.Vivaldi,
		useragent.InternetExplorer, useragent.SamsungBrowser, useragent.HeadlessChrome,
		"Huawei Browser", "Miui Browser", "Android browser":
		info.Browser = parsed.Name
		info.BrowserVersion = numericVersion(parsed.Version)
	}
	switch {
	case parsed.Bot:
		info.DeviceType = "bot"
	case parsed.Tablet:
		info.DeviceType = "tablet"
	case parsed.Mobile:
		info.DeviceType = "mobile"
	case parsed.Desktop:
		info.DeviceType = "desktop"
	}
	parts := make([]string, 0, 2)
	if info.Browser != "" {
		parts = append(parts, info.Browser)
	}
	if info.OS != "" {
		parts = append(parts, info.OS)
	}
	info.DisplayName = strings.Join(parts, " · ")
	if info.DisplayName == "" {
		info.DisplayName = "未知设备"
	}
	return info
}

func numericVersion(value string) string {
	for _, char := range value {
		if (char < '0' || char > '9') && char != '.' {
			return ""
		}
	}
	return value
}

func deviceLabel(parsed useragent.UserAgent) string {
	switch parsed.OS {
	case useragent.IOS:
		return parsed.Device
	case useragent.MacOS:
		return "Mac"
	default:
		return parsed.OS
	}
}
