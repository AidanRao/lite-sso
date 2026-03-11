package dto

import "sso-server/model"

// ToUserResponse converts a model.User to UserResponse DTO
func ToUserResponse(user *model.User) *UserResponse {
	if user == nil {
		return nil
	}
	return &UserResponse{
		ID:        user.ID,
		Email:     user.Email,
		Username:  user.Username,
		AvatarURL: user.AvatarURL,
	}
}
