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
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	ClientID    string `json:"client_id"`
	HomepageURL string `json:"homepage_url"`
	RedirectURI string `json:"redirect_uri"`
	LogoutURI   string `json:"logout_uri"`
}

// OAuthClientSecretResponse represents an OAuth client secret for administrators.
type OAuthClientSecretResponse struct {
	ID           uint   `json:"id"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

// CreateOAuthClientRequest represents the request body for creating an OAuth client.
type CreateOAuthClientRequest struct {
	Name         string `json:"name" binding:"required"`
	ClientID     string `json:"client_id" binding:"required"`
	ClientSecret string `json:"client_secret" binding:"required"`
	HomepageURL  string `json:"homepage_url" binding:"required"`
	RedirectURI  string `json:"redirect_uri" binding:"required"`
	LogoutURI    string `json:"logout_uri"`
}

// UpdateOAuthClientRequest represents the request body for updating an OAuth client.
type UpdateOAuthClientRequest struct {
	Name         string  `json:"name" binding:"required"`
	ClientID     string  `json:"client_id" binding:"required"`
	ClientSecret *string `json:"client_secret"`
	HomepageURL  string  `json:"homepage_url" binding:"required"`
	RedirectURI  string  `json:"redirect_uri" binding:"required"`
	LogoutURI    string  `json:"logout_uri"`
}
