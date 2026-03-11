package dto

// UserResponse represents user data returned in API responses
type UserResponse struct {
	ID        string  `json:"id"`
	Email     string  `json:"email"`
	Username  *string `json:"username"`
	AvatarURL *string `json:"avatar_url"`
}
