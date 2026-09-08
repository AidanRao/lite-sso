// Package feature evaluates and manages registered feature releases.
package feature

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"slices"

	"gorm.io/gorm"
	"sso-server/dal/db"
	"sso-server/dto"
	"sso-server/model"
)

// AuditLogs identifies the Profile audit log release.
const AuditLogs = "profile.audit_logs"

// ErrInvalidPolicy indicates an unknown feature or invalid release configuration.
var ErrInvalidPolicy = errors.New("invalid feature policy")

// Service is the shared source of release decisions.
type Service struct{ repo *db.FeatureRepository }

// NewService creates a database-backed release service without a policy cache.
func NewService(database *gorm.DB) *Service { return &Service{repo: db.NewFeatureRepository(database)} }

// Bucket assigns a stable, feature-specific bucket. This algorithm is a persistent contract.
func Bucket(key, userID string) uint64 {
	sum := sha256.Sum256([]byte(key + ":" + userID))
	return binary.BigEndian.Uint64(sum[:8]) % 10000
}

// Enabled evaluates a release policy without changing authentication or authorization.
func Enabled(config model.FeatureConfig, userID string) bool {
	if userID == "" {
		return false
	}
	switch config.Audience {
	case "all":
		return true
	case "selected", "percentage":
		for _, user := range config.Users {
			if user.UserID == userID {
				return true
			}
		}
		return config.Audience == "percentage" && config.Percentage >= 0 && config.Percentage <= 100 && Bucket(config.Key, userID) < uint64(config.Percentage*100)
	default:
		return false
	}
}

// List returns only features explicitly registered by the application.
func (s *Service) List(ctx context.Context) ([]dto.FeatureResponse, error) {
	configs, err := s.repo.Find(ctx)
	if err != nil {
		return nil, err
	}
	result := catalog()
	for i := range result {
		for _, config := range configs {
			if config.Key != result[i].Key {
				continue
			}
			result[i].Audience, result[i].Percentage, result[i].Stage = config.Audience, config.Percentage, config.Stage
			for _, user := range config.Users {
				result[i].UserIDs = append(result[i].UserIDs, user.UserID)
			}
		}
	}
	return result, nil
}

// States returns evaluated states only, excluding private release rules.
func (s *Service) States(ctx context.Context, userID string) (map[string]dto.FeatureState, error) {
	policies, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	states := make(map[string]dto.FeatureState, len(policies))
	for _, p := range policies {
		config := model.FeatureConfig{Key: p.Key, Audience: p.Audience, Percentage: p.Percentage, Stage: p.Stage}
		for _, id := range p.UserIDs {
			config.Users = append(config.Users, model.FeatureUser{FeatureKey: p.Key, UserID: id})
		}
		states[p.Key] = dto.FeatureState{Enabled: Enabled(config, userID), Stage: p.Stage}
	}
	return states, nil
}

// Update validates and atomically replaces a registered feature's release policy.
func (s *Service) Update(ctx context.Context, key string, req dto.FeatureUpdate) ([]string, error) {
	registered := slices.ContainsFunc(catalog(), func(item dto.FeatureResponse) bool { return item.Key == key })
	if !registered || !slices.Contains([]string{"off", "selected", "percentage", "all"}, req.Audience) || !slices.Contains([]string{"beta", "stable"}, req.Stage) || req.Percentage == nil || *req.Percentage < 0 || *req.Percentage > 100 || req.UserIDs == nil {
		return nil, ErrInvalidPolicy
	}
	ids := slices.Clone(req.UserIDs)
	slices.Sort(ids)
	ids = slices.Compact(ids)
	config := model.FeatureConfig{Key: key, Audience: req.Audience, Percentage: *req.Percentage, Stage: req.Stage}
	for _, id := range ids {
		if id == "" || len(id) > 36 {
			return nil, ErrInvalidPolicy
		}
		config.Users = append(config.Users, model.FeatureUser{FeatureKey: key, UserID: id})
	}
	changed, err := s.repo.Save(ctx, config)
	if errors.Is(err, db.ErrFeatureUserNotFound) {
		return nil, ErrInvalidPolicy
	}
	return changed, err
}

// catalog is the single registration point; new features start closed and in beta.
func catalog() []dto.FeatureResponse {
	return []dto.FeatureResponse{{Key: AuditLogs, Name: "操作日志", Audience: "off", Stage: "beta", UserIDs: []string{}}}
}
