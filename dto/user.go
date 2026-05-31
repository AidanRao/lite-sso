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
	LastLoginAt time.Time `json:"last_login_at"`
}

type ThirdPartyProviderResponse struct {
	Provider string `json:"provider"`
	Bound    bool   `json:"bound"`
}
