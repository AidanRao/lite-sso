package dto

import (
	"time"
)

// AdminUserResponse represents a user row in system administration.
type AdminUserResponse struct {
	ID        string    `json:"id"`
	Email     *string   `json:"email"`
	Username  *string   `json:"username"`
	AvatarURL *string   `json:"avatar_url"`
	Status    string    `json:"status"`
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
	ID            uint     `json:"id"`
	Name          string   `json:"name"`
	ClientID      string   `json:"client_id"`
	ClientType    string   `json:"client_type"`
	Audiences     []string `json:"audiences"`
	AllowedScopes []string `json:"allowed_scopes"`
	HomepageURL   string   `json:"homepage_url"`
	RedirectURIs  []string `json:"redirect_uris"`
	LogoutURI     string   `json:"logout_uri"`
	LogoURL       *string  `json:"logo_url"`
}

// OAuthClientSecretResponse represents an OAuth client secret for administrators.
type OAuthClientSecretResponse struct {
	ID           uint   `json:"id"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

// CreateOAuthClientRequest represents the request body for creating an OAuth client.
type CreateOAuthClientRequest struct {
	Name          string   `json:"name" binding:"required"`
	ClientID      string   `json:"client_id" binding:"required"`
	ClientSecret  string   `json:"client_secret"`
	ClientType    string   `json:"client_type"`
	Audiences     []string `json:"audiences"`
	AllowedScopes []string `json:"allowed_scopes"`
	HomepageURL   string   `json:"homepage_url" binding:"required"`
	RedirectURIs  []string `json:"redirect_uris" binding:"required,min=1"`
	LogoutURI     string   `json:"logout_uri"`
}

// UpdateOAuthClientRequest represents the request body for updating an OAuth client.
type UpdateOAuthClientRequest struct {
	Name          string   `json:"name" binding:"required"`
	ClientType    *string  `json:"client_type"`
	Audiences     []string `json:"audiences"`
	AllowedScopes []string `json:"allowed_scopes"`
	HomepageURL   string   `json:"homepage_url" binding:"required"`
	RedirectURIs  []string `json:"redirect_uris" binding:"required,min=1"`
	LogoutURI     string   `json:"logout_uri"`
}

// OAuthClientDetailResponse adds the number of associated users without secret metadata.
type OAuthClientDetailResponse struct {
	OAuthClientResponse
	UserCount int64 `json:"user_count"`
}

// OAuthClientUserResponse describes one client association.
type OAuthClientUserResponse struct {
	UserID      string    `json:"user_id"`
	Username    *string   `json:"username"`
	Email       *string   `json:"email"`
	AvatarURL   *string   `json:"avatar_url"`
	LastLoginAt time.Time `json:"last_login_at"`
}

// OAuthClientUsersPage is a bounded page of client associations.
type OAuthClientUsersPage struct {
	Users    []OAuthClientUserResponse `json:"users"`
	Total    int64                     `json:"total"`
	Page     int                       `json:"page"`
	PageSize int                       `json:"page_size"`
}

// OAuthClientUsersRemovalResponse reports the number of removed associations.
type OAuthClientUsersRemovalResponse struct {
	RemovedCount int64 `json:"removed_count"`
}
