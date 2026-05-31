// Package systemadmin contains system administration business logic.
package systemadmin

import (
	"context"
	"encoding/json"
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
	redirectURIs, err := normalizeURIs(req.RedirectURIs, true)
	if err != nil {
		return nil, err
	}
	logoutURIs, err := normalizeURIs(req.LogoutURIs, false)
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

	redirectRaw, err := marshalURIs(redirectURIs)
	if err != nil {
		return nil, err
	}
	logoutRaw, err := marshalURIs(logoutURIs)
	if err != nil {
		return nil, err
	}

	client := &model.OAuthClient{
		Name:         name,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURIs: redirectRaw,
		LogoutURIs:   logoutRaw,
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
	redirectURIs, err := normalizeURIs(req.RedirectURIs, true)
	if err != nil {
		return nil, err
	}
	logoutURIs, err := normalizeURIs(req.LogoutURIs, false)
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

	redirectRaw, err := marshalURIs(redirectURIs)
	if err != nil {
		return nil, err
	}
	logoutRaw, err := marshalURIs(logoutURIs)
	if err != nil {
		return nil, err
	}

	client.Name = name
	client.ClientID = clientID
	client.RedirectURIs = redirectRaw
	client.LogoutURIs = logoutRaw
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
		ID:           client.ID,
		Name:         client.Name,
		ClientID:     client.ClientID,
		RedirectURIs: parseURIList(client.RedirectURIs),
		LogoutURIs:   parseURIList(client.LogoutURIs),
	}
}

func normalizeURIs(values []string, required bool) ([]string, error) {
	result := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		parsed, err := url.ParseRequestURI(trimmed)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return nil, common.ErrInvalidOAuthClient
		}
		if !seen[trimmed] {
			result = append(result, trimmed)
			seen[trimmed] = true
		}
	}
	if required && len(result) == 0 {
		return nil, common.ErrInvalidOAuthClient
	}
	return result, nil
}

func marshalURIs(values []string) (string, error) {
	raw, err := json.Marshal(values)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func parseURIList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{}
	}

	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return []string{}
	}
	return values
}
