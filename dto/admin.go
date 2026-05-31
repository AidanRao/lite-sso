package dto

import "time"

// AdminUserResponse represents a user row in system administration.
type AdminUserResponse struct {
	ID        string    `json:"id"`
	Email     *string   `json:"email"`
	Username  *string   `json:"username"`
	AvatarURL *string   `json:"avatar_url"`
	IsActive  bool      `json:"is_active"`
	IsAdmin   bool      `json:"is_admin"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AdminUserDetailResponse represents user detail in system administration.
type AdminUserDetailResponse struct {
	User                *AdminUserResponse           `json:"user"`
	Applications        []UserApplicationResponse    `json:"applications"`
	ThirdPartyProviders []ThirdPartyProviderResponse `json:"third_party_providers"`
}

// OAuthClientResponse represents an OAuth client without exposing its secret.
type OAuthClientResponse struct {
	ID           uint     `json:"id"`
	Name         string   `json:"name"`
	ClientID     string   `json:"client_id"`
	RedirectURIs []string `json:"redirect_uris"`
	LogoutURIs   []string `json:"logout_uris"`
}

// CreateOAuthClientRequest represents the request body for creating an OAuth client.
type CreateOAuthClientRequest struct {
	Name         string   `json:"name" binding:"required"`
	ClientID     string   `json:"client_id" binding:"required"`
	ClientSecret string   `json:"client_secret" binding:"required"`
	RedirectURIs []string `json:"redirect_uris" binding:"required"`
	LogoutURIs   []string `json:"logout_uris"`
}

// UpdateOAuthClientRequest represents the request body for updating an OAuth client.
type UpdateOAuthClientRequest struct {
	Name         string   `json:"name" binding:"required"`
	ClientID     string   `json:"client_id" binding:"required"`
	ClientSecret *string  `json:"client_secret"`
	RedirectURIs []string `json:"redirect_uris" binding:"required"`
	LogoutURIs   []string `json:"logout_uris"`
}
