package user

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"sso-server/common"
	"sso-server/conf"
	"sso-server/dal/db"
	"sso-server/dto"
	"sso-server/model"
	serviceauth "sso-server/service/auth"
)

const (
	verificationTTL            = 30 * time.Minute
	verificationResendInterval = time.Minute
	verificationTemplateKey    = "email-address-verify"
	verificationURLVariable    = "verificationUrl"
	verificationRoute          = "/profile/access/emails/verify#token="
)

// EmailDeps contains user email service dependencies.
type EmailDeps struct {
	Config        *conf.Config
	DB            *gorm.DB
	MessageSender serviceauth.MessageSender
}

// EmailService coordinates email lifecycle operations.
type EmailService struct {
	cfg           *conf.Config
	db            *gorm.DB
	repo          *db.UserEmailRepository
	messageSender serviceauth.MessageSender
	now           func() time.Time
}

// NewEmailService creates a user email service.
func NewEmailService(deps EmailDeps) *EmailService {
	return &EmailService{
		cfg:           deps.Config,
		db:            deps.DB,
		repo:          db.NewUserEmailRepository(deps.DB),
		messageSender: deps.MessageSender,
		now:           time.Now,
	}
}

// List returns every address and the configured account limit.
func (s *EmailService) List(ctx context.Context, userID string) ([]dto.UserEmailResponse, int, error) {
	views, err := s.repo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, 0, err
	}
	result := make([]dto.UserEmailResponse, 0, len(views))
	for _, view := range views {
		result = append(result, toResponse(view))
	}
	return result, s.maxAddresses(), nil
}

// Add creates an unverified address, then attempts to send its verification link.
func (s *EmailService) Add(ctx context.Context, userID string, email string) (*dto.UserEmailResponse, error) {
	normalized, err := validateEmail(email)
	if err != nil {
		return nil, err
	}

	record := model.UserEmail{}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockUser(ctx, tx, userID); err != nil {
			return err
		}
		var ownCount int64
		if err := tx.Model(&model.UserEmail{}).Where("user_id = ? AND email = ?", userID, normalized).Count(&ownCount).Error; err != nil {
			return err
		}
		if ownCount > 0 {
			return common.ErrEmailAlreadyAdded
		}
		var verifiedCount int64
		if err := tx.Model(&model.UserEmail{}).Where("email = ? AND verified_at IS NOT NULL", normalized).Count(&verifiedCount).Error; err != nil {
			return err
		}
		if verifiedCount > 0 {
			return common.ErrEmailExists
		}
		var count int64
		if err := tx.Model(&model.UserEmail{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
			return err
		}
		if count >= int64(s.maxAddresses()) {
			return common.ErrEmailLimitReached
		}
		record = model.UserEmail{ID: uuid.NewString(), UserID: userID, Email: normalized}
		return tx.Create(&record).Error
	})
	if err != nil {
		if isUniqueConstraint(err, "uq_user_emails_user_email", "idx_user_email_owner") {
			return nil, common.ErrEmailAlreadyAdded
		}
		return nil, err
	}

	if _, err := s.sendVerification(ctx, userID, record.ID, true); err != nil {
		response := toResponse(db.UserEmailView{Email: record, Sources: []string{}})
		return &response, err
	}
	updated, err := s.repo.FindByIDForUser(ctx, userID, record.ID)
	if err != nil {
		return nil, err
	}
	response := toResponse(db.UserEmailView{Email: *updated, Sources: []string{}})
	return &response, nil
}

// Resend rotates an unverified address's token and sends a new link.
func (s *EmailService) Resend(ctx context.Context, userID string, emailID string) error {
	_, err := s.sendVerification(ctx, userID, emailID, false)
	return err
}

// Confirm verifies a one-time token for the currently authenticated user.
func (s *EmailService) Confirm(ctx context.Context, userID string, token string) (*dto.UserEmailResponse, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, common.ErrEmailVerificationInvalid
	}
	tokenHash := digestToken(token)
	var verified model.UserEmail
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockUser(ctx, tx, userID); err != nil {
			return err
		}
		var record model.UserEmail
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&record, "verification_token_hash = ?", tokenHash).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return common.ErrEmailVerificationInvalid
			}
			return err
		}
		if record.UserID != userID {
			return common.ErrEmailVerificationInvalid
		}
		if record.VerifiedAt != nil {
			return common.ErrEmailVerificationInvalid
		}
		now := s.now()
		if record.VerificationExpiresAt == nil || !now.Before(*record.VerificationExpiresAt) {
			return common.ErrEmailVerificationExpired
		}
		var conflicting int64
		if err := tx.Model(&model.UserEmail{}).
			Where("email = ? AND verified_at IS NOT NULL AND id <> ?", record.Email, record.ID).
			Count(&conflicting).Error; err != nil {
			return err
		}
		if conflicting > 0 {
			return common.ErrEmailExists
		}
		var primaryCount int64
		if err := tx.Model(&model.UserEmail{}).Where("user_id = ? AND is_primary = ?", userID, true).Count(&primaryCount).Error; err != nil {
			return err
		}
		record.VerifiedAt = &now
		record.IsPrimary = primaryCount == 0
		record.VerificationTokenHash = nil
		record.VerificationExpiresAt = nil
		if err := tx.Save(&record).Error; err != nil {
			return err
		}
		verified = record
		return nil
	})
	if err != nil {
		if isUniqueConstraint(err, "uq_user_emails_verified_email", "idx_user_email_verified") {
			return nil, common.ErrEmailExists
		}
		return nil, err
	}
	response := toResponse(db.UserEmailView{Email: verified, Sources: []string{}})
	return &response, nil
}

// SetPrimary makes a verified address the account's primary email.
func (s *EmailService) SetPrimary(ctx context.Context, userID string, emailID string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockUser(ctx, tx, userID); err != nil {
			return err
		}
		var record model.UserEmail
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&record, "id = ? AND user_id = ?", emailID, userID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return common.ErrEmailNotFound
			}
			return err
		}
		if record.VerifiedAt == nil {
			return common.ErrEmailNotVerified
		}
		if record.IsPrimary {
			return nil
		}
		if err := tx.Model(&model.UserEmail{}).Where("user_id = ? AND is_primary = ?", userID, true).Update("is_primary", false).Error; err != nil {
			return err
		}
		return tx.Model(&model.UserEmail{}).Where("id = ?", record.ID).Update("is_primary", true).Error
	})
}

// Delete removes an address while preserving at least one usable login method.
func (s *EmailService) Delete(ctx context.Context, userID string, emailID string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockUser(ctx, tx, userID); err != nil {
			return err
		}
		var record model.UserEmail
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&record, "id = ? AND user_id = ?", emailID, userID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return common.ErrEmailNotFound
			}
			return err
		}
		if record.IsPrimary {
			var otherVerifiedCount int64
			if err := tx.Model(&model.UserEmail{}).
				Where("user_id = ? AND id <> ? AND verified_at IS NOT NULL", userID, record.ID).
				Count(&otherVerifiedCount).Error; err != nil {
				return err
			}
			if otherVerifiedCount > 0 {
				return common.ErrPrimaryEmailDelete
			}
			var providerCount int64
			if err := tx.Model(&model.UserThirdParty{}).Where("user_id = ?", userID).Count(&providerCount).Error; err != nil {
				return err
			}
			if providerCount == 0 {
				return common.ErrLastLoginMethod
			}
		}
		return tx.Delete(&record).Error
	})
}

// ImportTrusted attaches a provider-asserted email during first account creation or binding.
// A capacity or ownership conflict intentionally skips import without failing the provider flow.
func (s *EmailService) ImportTrusted(ctx context.Context, userID string, email string, bindingID uint) (bool, error) {
	normalized, err := validateEmail(email)
	if err != nil {
		return false, nil
	}
	imported := false
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockUser(ctx, tx, userID); err != nil {
			return err
		}
		var record model.UserEmail
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&record, "user_id = ? AND email = ?", userID, normalized).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			var conflicting int64
			if err := tx.Model(&model.UserEmail{}).Where("email = ? AND verified_at IS NOT NULL", normalized).Count(&conflicting).Error; err != nil {
				return err
			}
			if conflicting > 0 {
				return nil
			}
			var count int64
			if err := tx.Model(&model.UserEmail{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
				return err
			}
			if count >= int64(s.maxAddresses()) {
				return nil
			}
			var primaryCount int64
			if err := tx.Model(&model.UserEmail{}).Where("user_id = ? AND is_primary = ?", userID, true).Count(&primaryCount).Error; err != nil {
				return err
			}
			now := s.now()
			record = model.UserEmail{ID: uuid.NewString(), UserID: userID, Email: normalized, VerifiedAt: &now, IsPrimary: primaryCount == 0}
			if err := tx.Create(&record).Error; err != nil {
				return err
			}
		} else if record.VerifiedAt == nil {
			var conflicting int64
			if err := tx.Model(&model.UserEmail{}).Where("email = ? AND verified_at IS NOT NULL AND id <> ?", normalized, record.ID).Count(&conflicting).Error; err != nil {
				return err
			}
			if conflicting > 0 {
				return nil
			}
			now := s.now()
			updates := map[string]interface{}{"verified_at": now, "verification_token_hash": nil, "verification_expires_at": nil}
			if err := tx.Model(&record).Updates(updates).Error; err != nil {
				return err
			}
		}
		source := model.UserEmailSource{UserEmailID: record.ID, UserThirdPartyID: bindingID}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&source).Error; err != nil {
			return err
		}
		imported = true
		return nil
	})
	if isUniqueConstraint(err, "uq_user_emails_verified_email", "idx_user_email_verified") {
		return false, nil
	}
	return imported, err
}

func (s *EmailService) sendVerification(ctx context.Context, userID string, emailID string, bypassCooldown bool) (string, error) {
	var record model.UserEmail
	var token string
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&record, "id = ? AND user_id = ?", emailID, userID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return common.ErrEmailNotFound
			}
			return err
		}
		if record.VerifiedAt != nil {
			return common.ErrEmailNotVerified
		}
		now := s.now()
		if !bypassCooldown && record.VerificationSentAt != nil && now.Sub(*record.VerificationSentAt) < verificationResendInterval {
			return common.ErrEmailVerificationResend
		}
		generated, err := generateToken()
		if err != nil {
			return err
		}
		token = generated
		hash := digestToken(token)
		expiresAt := now.Add(verificationTTL)
		record.VerificationTokenHash = &hash
		record.VerificationExpiresAt = &expiresAt
		record.VerificationSentAt = &now
		return tx.Save(&record).Error
	})
	if err != nil {
		return "", err
	}
	if s.skipMessageSend() {
		return token, nil
	}
	if s.messageSender == nil {
		return "", common.ErrMessageNotSent
	}
	verificationURL := strings.TrimRight(strings.TrimSpace(s.cfg.Email.VerificationBaseURL), "/") + verificationRoute + token
	if err := s.messageSender.Send(ctx, record.Email, verificationTemplateKey, map[string]string{verificationURLVariable: verificationURL}); err != nil {
		return "", fmt.Errorf("%w: %v", common.ErrMessageNotSent, err)
	}
	return token, nil
}

func (s *EmailService) skipMessageSend() bool {
	return s.cfg != nil && conf.GetEnvironmentName() == string(conf.EnvLocal) && s.cfg.Dev.SkipSendMessage
}

func (s *EmailService) maxAddresses() int {
	if s.cfg == nil || s.cfg.Email.MaxAddresses <= 0 {
		return 3
	}
	return s.cfg.Email.MaxAddresses
}

func lockUser(ctx context.Context, tx *gorm.DB, userID string) error {
	var user model.User
	if err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").First(&user, "id = ?", userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return common.ErrUserNotFound
		}
		return err
	}
	return nil
}

func validateEmail(value string) (string, error) {
	normalized := db.NormalizeEmail(value)
	if normalized == "" || len(normalized) > 100 {
		return "", common.ErrEmailInvalid
	}
	parsed, err := mail.ParseAddress(normalized)
	if err != nil || parsed.Address != normalized {
		return "", common.ErrEmailInvalid
	}
	return normalized, nil
}

func generateToken() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func digestToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func isUniqueConstraint(err error, names ...string) bool {
	if err == nil {
		return false
	}
	var postgresError *pgconn.PgError
	if !errors.As(err, &postgresError) || postgresError.Code != "23505" {
		return false
	}
	for _, name := range names {
		if postgresError.ConstraintName == name {
			return true
		}
	}
	return false
}

func toResponse(view db.UserEmailView) dto.UserEmailResponse {
	sources := view.Sources
	if sources == nil {
		sources = []string{}
	}
	return dto.UserEmailResponse{
		ID:         view.Email.ID,
		Email:      view.Email.Email,
		Verified:   view.Email.VerifiedAt != nil,
		IsPrimary:  view.Email.IsPrimary,
		Sources:    sources,
		CreatedAt:  view.Email.CreatedAt,
		VerifiedAt: view.Email.VerifiedAt,
	}
}
