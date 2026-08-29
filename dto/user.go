package dto

import "time"

const (
	LoginMethodEmailOTP   LoginMethodType = "email_otp"
	LoginMethodPassword   LoginMethodType = "password"
	LoginMethodThirdParty LoginMethodType = "third_party"
)

// UserResponse represents user data returned in API responses.
type UserResponse struct {
	ID        string  `json:"id"`
	Email     *string `json:"email"`
	Username  *string `json:"username"`
	AvatarURL *string `json:"avatar_url"`
}

// ProfileResponse represents the current user's account summary.
type ProfileResponse struct {
	User    *UserResponse `json:"user"`
	IsAdmin bool          `json:"is_admin"`
}

// LoginMethodType identifies a supported sign-in method category.
type LoginMethodType string

// LoginMethodResponse describes one sign-in method available to the current user.
type LoginMethodResponse struct {
	Type     LoginMethodType `json:"type"`
	Email    *string         `json:"email,omitempty"`
	Provider string          `json:"provider,omitempty"`
	Bound    *bool           `json:"bound,omitempty"`
}

// UserApplicationResponse describes an OAuth application used by a user.
type UserApplicationResponse struct {
	ClientID    string    `json:"client_id"`
	Name        string    `json:"name"`
	HomepageURL string    `json:"homepage_url"`
	LogoURL     *string   `json:"logo_url"`
	LastLoginAt time.Time `json:"last_login_at"`
}

// ThirdPartyProviderResponse describes an administrator-visible provider binding.
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
