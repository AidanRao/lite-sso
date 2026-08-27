package dto

import "time"

// UserResponse represents user data returned in API responses
type UserResponse struct {
	ID        string  `json:"id"`
	Email     *string `json:"email"`
	Username  *string `json:"username"`
	AvatarURL *string `json:"avatar_url"`
}

type ProfileResponse struct {
	User                *UserResponse                `json:"user"`
	Applications        []UserApplicationResponse    `json:"applications"`
	ThirdPartyProviders []ThirdPartyProviderResponse `json:"third_party_providers"`
	IsAdmin             bool                         `json:"is_admin"`
}

type UserApplicationResponse struct {
	ClientID    string    `json:"client_id"`
	Name        string    `json:"name"`
	HomepageURL string    `json:"homepage_url"`
	LogoURL     *string   `json:"logo_url"`
	LastLoginAt time.Time `json:"last_login_at"`
}

type ThirdPartyProviderResponse struct {
	Provider string `json:"provider"`
	Bound    bool   `json:"bound"`
}

// LoginDeviceResponse describes one active browser device.
type LoginDeviceResponse struct {
	DeviceID   string    `json:"device_id"`
	UserAgent  string    `json:"user_agent"`
	IP         string    `json:"ip"`
	AuthMethod string    `json:"auth_method"`
	CreatedAt  time.Time `json:"created_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	Current    bool      `json:"current"`
}
