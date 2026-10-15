package db

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"sso-server/common"
	"sso-server/model"
)

// DatabaseTime reads wall-clock time after acquiring locks, including long-running transactions.
func DatabaseTime(ctx context.Context, database *gorm.DB) (time.Time, error) {
	if database.Dialector.Name() != "postgres" {
		return time.Now(), nil
	}
	var now time.Time
	err := database.WithContext(ctx).Raw("SELECT clock_timestamp()").Scan(&now).Error
	return now.UTC(), err
}

// LockUser serializes lifecycle changes and credential or profile writes.
func LockUser(ctx context.Context, tx *gorm.DB, userID string) (*model.User, error) {
	var user model.User
	if err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, "id = ?", userID).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// LockActiveUser also prevents writes after suspension or a deletion request.
func LockActiveUser(ctx context.Context, tx *gorm.DB, userID string) (*model.User, error) {
	user, err := LockUser(ctx, tx, userID)
	if err != nil {
		return nil, err
	}
	if user.Status == model.UserStatusDisabled {
		return nil, common.ErrUserInactive
	}
	if user.Status != model.UserStatusActive {
		return nil, common.ErrAccountPendingDeletion
	}
	return user, nil
}

// ValidateUserSession checks both the account and the purpose of a persistent session.
func ValidateUserSession(ctx context.Context, database *gorm.DB, user *model.User, session *model.UserSession, now time.Time) error {
	if (user.Status != model.UserStatusActive && user.Status != model.UserStatusPendingDeletion) || session.UserID != user.ID || session.RevokedAt != nil || !now.Before(session.ExpiresAt) {
		return common.ErrSessionRevoked
	}
	if user.Status == model.UserStatusPendingDeletion {
		request, err := PendingUserDeletion(ctx, database, user.ID)
		if err != nil {
			return err
		}
		if session.Purpose != model.SessionPurposeAccountDeletion || session.DeletionID == nil || *session.DeletionID != request.ID {
			return common.ErrSessionRevoked
		}
	} else if session.Purpose == model.SessionPurposeAccountDeletion {
		return common.ErrSessionRevoked
	}
	return nil
}

// LatestUserDeletion loads lifecycle history only when the caller needs it.
func LatestUserDeletion(ctx context.Context, database *gorm.DB, userID string) (*model.UserDeletion, error) {
	var request model.UserDeletion
	query := database.WithContext(ctx).Where("user_id = ?", userID).Order("requested_at DESC, id DESC").Limit(1).Find(&request)
	if query.Error != nil {
		return nil, query.Error
	}
	if query.RowsAffected == 0 {
		return nil, nil
	}
	return &request, nil
}

// PendingUserDeletion requires a live pending request for a restricted account.
func PendingUserDeletion(ctx context.Context, database *gorm.DB, userID string) (*model.UserDeletion, error) {
	request, err := LatestUserDeletion(ctx, database, userID)
	if err != nil {
		return nil, err
	}
	if request == nil || request.Status != "pending" {
		return nil, common.ErrSessionRevoked
	}
	return request, nil
}

// CheckUserRevocation rejects proofs or refresh grants predating any deletion request.
// Cancellation never removes this cutoff; newly created users have independent IDs.
func CheckUserRevocation(ctx context.Context, database *gorm.DB, userID string, authorizedAt int64) error {
	request, err := LatestUserDeletion(ctx, database, userID)
	if err != nil {
		return err
	}
	if request != nil && authorizedAt <= request.RequestedAt.UnixMicro() {
		return common.ErrSessionRevoked
	}
	return nil
}

// PurgeExpiredIdentities releases expired accounts before identity lookup or uniqueness checks.
// Each identity condition is a parameterized users predicate, never user-provided SQL.
func PurgeExpiredIdentities(ctx context.Context, database *gorm.DB, predicate string, args ...any) error {
	now, err := DatabaseTime(ctx, database)
	if err != nil {
		return err
	}
	var ids []string
	if err := database.WithContext(ctx).Model(&model.User{}).Where("status = ? AND id IN (?)", model.UserStatusPendingDeletion, database.Model(&model.UserDeletion{}).Select("user_id").Where("status = ? AND scheduled_at <= ?", "pending", now)).Where(predicate, args...).Pluck("id", &ids).Error; err != nil {
		return err
	}
	for _, id := range ids {
		if err := database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			user, err := LockUser(ctx, tx, id)
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			if err != nil {
				return err
			}
			now, err := DatabaseTime(ctx, tx)
			if err != nil {
				return err
			}
			if user.Status != model.UserStatusPendingDeletion {
				return nil
			}
			request, err := PendingUserDeletion(ctx, tx, user.ID)
			if err != nil {
				return err
			}
			if !request.Due(now) {
				return nil
			}
			return PurgeAccount(ctx, tx, user, request, now)
		}); err != nil {
			return err
		}
	}
	return nil
}

// PurgeAccount deletes database identities atomically and persists the owned-avatar cleanup.
// The caller must hold the users row lock and verify the deadline.
func PurgeAccount(ctx context.Context, tx *gorm.DB, user *model.User, request *model.UserDeletion, now time.Time) error {
	if user.AvatarObjectKey != nil && *user.AvatarObjectKey != "" {
		task := model.UserCleanupTask{ID: uuid.NewString(), UserID: user.ID, Kind: "avatar", ObjectKey: *user.AvatarObjectKey, Cutoff: now, NextAttemptAt: now}
		if err := tx.WithContext(ctx).Create(&task).Error; err != nil {
			return err
		}
	}
	if err := tx.WithContext(ctx).Where("user_email_id IN (?)", tx.Model(&model.UserEmail{}).Select("id").Where("user_id = ?", user.ID)).Delete(&model.UserEmailSource{}).Error; err != nil {
		return err
	}
	for _, record := range []any{&model.UserEmail{}, &model.UserThirdParty{}, &model.UserOAuthClient{}, &model.WebAuthnCredential{}, &model.WebAuthnUser{}, &model.UserSession{}, &model.FeatureUser{}} {
		if err := tx.WithContext(ctx).Where("user_id = ?", user.ID).Delete(record).Error; err != nil {
			return err
		}
	}
	if err := tx.WithContext(ctx).Model(&model.UserDeletion{}).Where("id = ? AND status = ?", request.ID, "pending").Updates(map[string]any{"status": "completed", "finished_at": now}).Error; err != nil {
		return err
	}
	event := model.AuditLog{ID: uuid.NewString(), OccurredAt: now, CreatedAt: now, UserID: &user.ID, ActorType: "user", Action: "user.account.deletion.complete", TargetType: "user", TargetID: user.ID, Method: "TASK", Route: "account-deletion/cleanup", HTTPStatus: 200, Outcome: "success", ReasonCode: "OK", Details: "{}"}
	if err := tx.WithContext(ctx).Create(&event).Error; err != nil {
		return err
	}
	return tx.WithContext(ctx).Delete(&model.User{}, "id = ?", user.ID).Error
}
