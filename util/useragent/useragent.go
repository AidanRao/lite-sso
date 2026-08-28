// Package useragent extracts reusable, best-effort device information from User-Agent values.
package useragent

import "strings"

// DeviceFamily identifies a broad device or operating-system family.
type DeviceFamily string

const (
	DeviceUnknown  DeviceFamily = ""
	DeviceIPhone   DeviceFamily = "iphone"
	DeviceIPad     DeviceFamily = "ipad"
	DeviceAndroid  DeviceFamily = "android"
	DeviceWindows  DeviceFamily = "windows"
	DeviceChromeOS DeviceFamily = "chromeos"
	DeviceMac      DeviceFamily = "mac"
	DeviceLinux    DeviceFamily = "linux"
)

// DeviceInfo contains display-oriented information inferred from a User-Agent.
// It must not be used for authentication or authorization decisions.
type DeviceInfo struct {
	Family DeviceFamily
	Label  string
}

// Parse returns a coarse device family and display label. Unknown and reduced
// User-Agent values intentionally return the zero value.
func Parse(value string) DeviceInfo {
	switch normalized := strings.ToLower(value); {
	case strings.Contains(normalized, "iphone"):
		return DeviceInfo{Family: DeviceIPhone, Label: "iPhone"}
	case strings.Contains(normalized, "ipad"):
		return DeviceInfo{Family: DeviceIPad, Label: "iPad"}
	case strings.Contains(normalized, "android"):
		return DeviceInfo{Family: DeviceAndroid, Label: "Android"}
	case strings.Contains(normalized, "windows"):
		return DeviceInfo{Family: DeviceWindows, Label: "Windows"}
	case strings.Contains(normalized, "cros"):
		return DeviceInfo{Family: DeviceChromeOS, Label: "ChromeOS"}
	case strings.Contains(normalized, "macintosh"), strings.Contains(normalized, "mac os x"):
		return DeviceInfo{Family: DeviceMac, Label: "Mac"}
	case strings.Contains(normalized, "linux"):
		return DeviceInfo{Family: DeviceLinux, Label: "Linux"}
	default:
		return DeviceInfo{}
	}
}
