// Package systemadmin contains system administration business logic.
package systemadmin

import (
	"context"
	"net/url"
	"strings"

	"gorm.io/gorm"

	"sso-server/common"
	"sso-server/conf"
	"sso-server/dal/db"
	"sso-server/dto"
	"sso-server/model"
)

type AdminService struct {
	cfg                *conf.Config
	userRepo           *db.UserRepository
	clientRepo         *db.OAuthClientRepository
	userOAuthRepo      *db.UserOAuthClientRepository
	userThirdPartyRepo *db.UserThirdPartyRepository
}

// NewAdminService creates a service for system administration workflows.
func NewAdminService(cfg *conf.Config, database *gorm.DB) *AdminService {
	return &AdminService{
		cfg:                cfg,
		userRepo:           db.NewUserRepository(database),
		clientRepo:         db.NewOAuthClientRepository(database),
		userOAuthRepo:      db.NewUserOAuthClientRepository(database),
		userThirdPartyRepo: db.NewUserThirdPartyRepository(database),
	}
}

// ListUsers returns all system users with administrator markers.
func (s *AdminService) ListUsers(ctx context.Context) ([]dto.AdminUserResponse, error) {
	users, err := s.userRepo.FindAll(ctx)
	if err != nil {
		return nil, err
	}

	responses := make([]dto.AdminUserResponse, 0, len(users))
	for _, user := range users {
		responses = append(responses, *toAdminUserResponse(&user, s.cfg))
	}
	return responses, nil
}

// GetUserDetail returns a user's profile overview for administrators.
func (s *AdminService) GetUserDetail(ctx context.Context, userID string) (*dto.AdminUserDetailResponse, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, common.ErrUserNotFound
	}

	apps, err := s.userOAuthRepo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	bindings, err := s.userThirdPartyRepo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	boundProviders := make(map[string]bool, len(bindings))
	for _, binding := range bindings {
		boundProviders[binding.Provider] = true
	}

	appResponses := make([]dto.UserApplicationResponse, 0, len(apps))
	for _, app := range apps {
		appResponses = append(appResponses, dto.UserApplicationResponse{
			ClientID:    app.ClientID,
			Name:        app.Name,
			LastLoginAt: app.LastLoginAt,
		})
	}

	providerResponses := []dto.ThirdPartyProviderResponse{
		{Provider: "github", Bound: boundProviders["github"]},
		{Provider: "feishu", Bound: boundProviders["feishu"]},
	}

	return &dto.AdminUserDetailResponse{
		User:                toAdminUserResponse(user, s.cfg),
		Applications:        appResponses,
		ThirdPartyProviders: providerResponses,
	}, nil
}

// ListOAuthClients returns configured OAuth clients for connected platforms.
func (s *AdminService) ListOAuthClients(ctx context.Context) ([]dto.OAuthClientResponse, error) {
	clients, err := s.clientRepo.FindAll(ctx)
	if err != nil {
		return nil, err
	}

	responses := make([]dto.OAuthClientResponse, 0, len(clients))
	for _, client := range clients {
		responses = append(responses, toOAuthClientResponse(&client))
	}
	return responses, nil
}

// GetOAuthClientSecret returns an OAuth client secret for administrators.
func (s *AdminService) GetOAuthClientSecret(ctx context.Context, id uint) (*dto.OAuthClientSecretResponse, error) {
	client, err := s.clientRepo.FindByID(ctx, id)
	if err != nil {
		return nil, common.ErrOAuthClientNotFound
	}

	return &dto.OAuthClientSecretResponse{
		ID:           client.ID,
		ClientID:     client.ClientID,
		ClientSecret: client.ClientSecret,
	}, nil
}

func toAdminUserResponse(user *model.User, cfg *conf.Config) *dto.AdminUserResponse {
	return &dto.AdminUserResponse{
		ID:        user.ID,
		Email:     user.Email,
		Username:  user.Username,
		AvatarURL: user.AvatarURL,
		IsActive:  user.IsActive,
		IsAdmin:   cfg.IsAdminUser(user.ID),
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}

// CreateOAuthClient validates and creates an OAuth client.
func (s *AdminService) CreateOAuthClient(ctx context.Context, req dto.CreateOAuthClientRequest) (*dto.OAuthClientResponse, error) {
	name := strings.TrimSpace(req.Name)
	clientID := strings.TrimSpace(req.ClientID)
	clientSecret := strings.TrimSpace(req.ClientSecret)
	homepageURL, err := normalizeURI(req.HomepageURL)
	if err != nil {
		return nil, err
	}
	redirectURI, err := normalizeURI(req.RedirectURI)
	if err != nil {
		return nil, err
	}
	logoutURI, err := normalizeOptionalURI(req.LogoutURI)
	if err != nil {
		return nil, err
	}
	if name == "" || clientID == "" || clientSecret == "" {
		return nil, common.ErrInvalidOAuthClient
	}

	exists, err := s.clientRepo.ExistsClientID(ctx, clientID, 0)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, common.ErrOAuthClientExists
	}

	client := &model.OAuthClient{
		Name:         name,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		HomepageURL:  homepageURL,
		RedirectURI:  redirectURI,
		LogoutURI:    logoutURI,
	}
	if err := s.clientRepo.Create(ctx, client); err != nil {
		return nil, err
	}
	response := toOAuthClientResponse(client)
	return &response, nil
}

// UpdateOAuthClient validates and updates an OAuth client.
func (s *AdminService) UpdateOAuthClient(ctx context.Context, id uint, req dto.UpdateOAuthClientRequest) (*dto.OAuthClientResponse, error) {
	client, err := s.clientRepo.FindByID(ctx, id)
	if err != nil {
		return nil, common.ErrOAuthClientNotFound
	}

	name := strings.TrimSpace(req.Name)
	clientID := strings.TrimSpace(req.ClientID)
	homepageURL, err := normalizeURI(req.HomepageURL)
	if err != nil {
		return nil, err
	}
	redirectURI, err := normalizeURI(req.RedirectURI)
	if err != nil {
		return nil, err
	}
	logoutURI, err := normalizeOptionalURI(req.LogoutURI)
	if err != nil {
		return nil, err
	}
	if name == "" || clientID == "" {
		return nil, common.ErrInvalidOAuthClient
	}

	exists, err := s.clientRepo.ExistsClientID(ctx, clientID, id)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, common.ErrOAuthClientExists
	}

	client.Name = name
	client.ClientID = clientID
	client.HomepageURL = homepageURL
	client.RedirectURI = redirectURI
	client.LogoutURI = logoutURI
	if req.ClientSecret != nil && strings.TrimSpace(*req.ClientSecret) != "" {
		client.ClientSecret = strings.TrimSpace(*req.ClientSecret)
	}

	if err := s.clientRepo.Update(ctx, client); err != nil {
		return nil, err
	}
	response := toOAuthClientResponse(client)
	return &response, nil
}

func toOAuthClientResponse(client *model.OAuthClient) dto.OAuthClientResponse {
	return dto.OAuthClientResponse{
		ID:          client.ID,
		Name:        client.Name,
		ClientID:    client.ClientID,
		HomepageURL: client.HomepageURL,
		RedirectURI: client.RedirectURI,
		LogoutURI:   client.LogoutURI,
	}
}

func normalizeURI(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	parsed, err := url.ParseRequestURI(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", common.ErrInvalidOAuthClient
	}
	return trimmed, nil
}

func normalizeOptionalURI(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nil
	}
	return normalizeURI(trimmed)
}
