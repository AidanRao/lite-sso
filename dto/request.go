package dto

// RegisterRequest represents the request body for user registration
type RegisterRequest struct {
	Email    string  `json:"email" binding:"required,email"`
	Password string  `json:"password" binding:"required"`
	Username *string `json:"username"`
	OTP      string  `json:"otp" binding:"required"`
}

// UpdateProfileRequest represents the request body for updating user profile
type UpdateProfileRequest struct {
	Username  *string `json:"username"`
	AvatarURL *string `json:"avatar_url"`
}

// SendEmailOTPRequest represents the request body for sending email OTP
type SendEmailOTPRequest struct {
	Email     string `json:"email" binding:"required,email"`
	CaptchaID string `json:"captcha_id" binding:"required"`
	Captcha   string `json:"captcha" binding:"required"`
}

// BindThirdPartyRequest represents the request body for binding third-party account
type BindThirdPartyRequest struct {
	Provider string `json:"provider" binding:"required"`
}
