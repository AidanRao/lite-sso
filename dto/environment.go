package dto

import (
	"sso-server/util/requestmeta"
)

// EnvironmentResponse is a display-only description of a login request.
// Its client-reported device information must not be used for authorization.
type EnvironmentResponse struct {
	Browser        string `json:"browser"`
	BrowserVersion string `json:"browser_version"`
	OS             string `json:"os"`
	OSVersion      string `json:"os_version"`
	DeviceType     string `json:"device_type"`
	DeviceName     string `json:"device_name"`
	DisplayName    string `json:"display_name"`
	IP             string `json:"ip"`
	Location       string `json:"location"`
}

// ToEnvironmentResponse maps parsed environment information to the public response.
func ToEnvironmentResponse(environment requestmeta.Environment) EnvironmentResponse {
	return EnvironmentResponse{
		Browser: environment.Browser, BrowserVersion: environment.BrowserVersion,
		OS: environment.OS, OSVersion: environment.OSVersion, DeviceType: environment.DeviceType,
		DeviceName: environment.DeviceName, DisplayName: environment.DisplayName, IP: environment.IP, Location: environment.Location,
	}
}
