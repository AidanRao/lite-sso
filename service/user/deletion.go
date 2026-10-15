package user

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"sso-server/common"
	"sso-server/dal/db"
	"sso-server/dto"
	"sso-server/manager/oauth"
	"sso-server/manager/oss"
	"sso-server/model"
)

const (
	// DeletionCoolingPeriod defines the delay before an account is permanently deleted.
	// DeletionCoolingPeriod = 24 * time.Hour
	DeletionCoolingPeriod = 1 * time.Minute
	scanInterval          = time.Minute
	batchSize             = 100
)

// DeletionService owns account lifecycle transactions and the restartable cleanup worker.
type DeletionService struct {
	database *gorm.DB
	refresh  oauth.RefreshStore
	images   oss.ImageStore
	cancel   context.CancelFunc
	workers  sync.WaitGroup
}

// NewDeletionService creates an account lifecycle service without starting background work.
func NewDeletionService(database *gorm.DB, refresh oauth.RefreshStore, images oss.ImageStore) *DeletionService {
	return &DeletionService{database: database, refresh: refresh, images: images}
}

// Status returns a pending request without exposing identity or credential fields.
func (s *DeletionService) Status(ctx context.Context, userID string) (*dto.AccountDeletionResponse, error) {
	user, err := db.NewUserRepository(s.database).FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	now, err := db.DatabaseTime(ctx, s.database)
	if err != nil {
		return nil, err
	}
	return s.status(ctx, s.database, user, now)
}

// Request atomically starts a cooling period and invalidates every existing session.
func (s *DeletionService) Request(ctx context.Context, userID, sessionID string) (*dto.AccountDeletionResponse, error) {
	var result *dto.AccountDeletionResponse
	err := s.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		user, err := db.LockUser(ctx, tx, userID)
		if err != nil {
			return err
		}
		now, err := db.DatabaseTime(ctx, tx)
		if err != nil {
			return err
		}
		session, err := db.NewUserSessionRepository(tx).FindActive(ctx, sessionID, now)
		if err != nil {
			return common.ErrSessionRevoked
		}
		if err := db.ValidateUserSession(ctx, tx, user, session, now); err != nil {
			return err
		}
		if user.Status == model.UserStatusPendingDeletion {
			result, err = s.status(ctx, tx, user, now)
			return err
		}
		request := model.UserDeletion{ID: uuid.NewString(), UserID: userID, RequestedAt: now, ScheduledAt: now.Add(DeletionCoolingPeriod), Status: "pending"}
		if err := tx.Create(&request).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.User{}).Where("id = ?", userID).Updates(map[string]any{"status": model.UserStatusPendingDeletion}).Error; err != nil {
			return err
		}
		if err := db.NewUserSessionRepository(tx).RevokeAll(ctx, userID, "account_deletion", now); err != nil {
			return err
		}
		if err := tx.Model(&model.UserEmail{}).Where("user_id = ?", userID).Updates(map[string]any{"verification_token_hash": nil, "verification_expires_at": nil}).Error; err != nil {
			return err
		}
		task := model.UserCleanupTask{ID: uuid.NewString(), UserID: userID, Kind: "oauth_refresh", Cutoff: now, NextAttemptAt: now}
		if err := tx.Create(&task).Error; err != nil {
			return err
		}
		result = &dto.AccountDeletionResponse{Status: model.UserStatusPendingDeletion, RequestedAt: &request.RequestedAt, ScheduledAt: &request.ScheduledAt, CanCancel: true}
		return nil
	})
	return result, err
}

// Cancel restores account access before the deadline without reviving old sessions or grants.
func (s *DeletionService) Cancel(ctx context.Context, userID, sessionID string) error {
	return s.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		user, err := db.LockUser(ctx, tx, userID)
		if err != nil {
			return err
		}
		now, err := db.DatabaseTime(ctx, tx)
		if err != nil {
			return err
		}
		session, err := db.NewUserSessionRepository(tx).FindActive(ctx, sessionID, now)
		if err != nil {
			return common.ErrSessionRevoked
		}
		if err := db.ValidateUserSession(ctx, tx, user, session, now); err != nil {
			return err
		}
		if user.Status != model.UserStatusPendingDeletion {
			return nil
		}
		request, err := db.PendingUserDeletion(ctx, tx, user.ID)
		if err != nil {
			return err
		}
		if request.Due(now) {
			return common.ErrAccountDeletionDue
		}
		if err := tx.Model(&model.UserDeletion{}).Where("id = ? AND status = ?", request.ID, "pending").Updates(map[string]any{"status": "cancelled", "finished_at": now}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.User{}).Where("id = ?", userID).Updates(map[string]any{"status": model.UserStatusActive}).Error; err != nil {
			return err
		}
		return db.NewUserSessionRepository(tx).RevokeAll(ctx, userID, "account_deletion_cancelled", now)
	})
}

// Start runs an immediate catch-up pass followed by bounded minute scans.
func (s *DeletionService) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.workers.Add(1)
	go func() {
		defer s.workers.Done()
		ticker := time.NewTicker(scanInterval)
		defer ticker.Stop()
		for {
			runCtx, stop := context.WithTimeout(ctx, 45*time.Second)
			if err := s.RunOnce(runCtx); err != nil && ctx.Err() == nil {
				log.Printf("account cleanup failed: %v", err)
			}
			stop()
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

// Shutdown stops scanning and waits for the currently running transaction to finish.
func (s *DeletionService) Shutdown(ctx context.Context) error {
	if s.cancel == nil {
		return nil
	}
	s.cancel()
	done := make(chan struct{})
	go func() { s.workers.Wait(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// RunOnce processes bounded batches; row locks make each batch safe across instances.
func (s *DeletionService) RunOnce(ctx context.Context) error {
	for range batchSize {
		found := false
		err := s.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			now, err := db.DatabaseTime(ctx, tx)
			if err != nil {
				return err
			}
			var user model.User
			query := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).Where("status = ? AND id IN (?)", model.UserStatusPendingDeletion, tx.Model(&model.UserDeletion{}).Select("user_id").Where("status = ? AND scheduled_at <= ?", "pending", now)).Order("id").Limit(1).Find(&user)
			err = query.Error
			if err == nil && query.RowsAffected == 0 {
				return nil
			}
			if err != nil {
				return err
			}
			found = true
			request, err := db.PendingUserDeletion(ctx, tx, user.ID)
			if err != nil {
				return err
			}
			if !request.Due(now) {
				return nil
			}
			return db.PurgeAccount(ctx, tx, &user, request, now)
		})
		if err != nil {
			return err
		}
		if !found {
			break
		}
	}
	for range batchSize {
		found := false
		err := s.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			now, err := db.DatabaseTime(ctx, tx)
			if err != nil {
				return err
			}
			var task model.UserCleanupTask
			query := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).Where("next_attempt_at <= ?", now).Order("next_attempt_at").Limit(1).Find(&task)
			err = query.Error
			if err == nil && query.RowsAffected == 0 {
				return nil
			}
			if err != nil {
				return err
			}
			found = true
			taskCtx, stop := context.WithTimeout(ctx, 10*time.Second)
			defer stop()
			switch task.Kind {
			case "oauth_refresh":
				if s.refresh == nil {
					err = errors.New("OAuth refresh store unavailable")
				} else {
					err = s.refresh.RevokeUserBefore(taskCtx, task.UserID, task.Cutoff)
				}
			case "avatar":
				if s.images == nil {
					err = errors.New("image store unavailable")
				} else {
					err = s.images.DeleteImage(taskCtx, task.ObjectKey)
				}
			default:
				err = errors.New("unknown account cleanup task")
			}
			if err == nil {
				return tx.Delete(&task).Error
			}
			log.Printf("account cleanup retry: kind=%s attempt=%d", task.Kind, task.Attempts+1)
			delay := time.Minute * time.Duration(1<<min(task.Attempts, 6))
			return tx.Model(&task).Updates(map[string]any{"attempts": task.Attempts + 1, "next_attempt_at": now.Add(delay)}).Error
		})
		if err != nil {
			return err
		}
		if !found {
			break
		}
	}
	now, err := db.DatabaseTime(ctx, s.database)
	if err != nil {
		return err
	}
	return s.database.WithContext(ctx).Where("finished_at < ? AND user_id NOT IN (?)", now.AddDate(0, 0, -90), s.database.Model(&model.User{}).Select("id")).Delete(&model.UserDeletion{}).Error
}

func (s *DeletionService) status(ctx context.Context, database *gorm.DB, user *model.User, now time.Time) (*dto.AccountDeletionResponse, error) {
	result := &dto.AccountDeletionResponse{Status: user.Status}
	if user.Status != model.UserStatusPendingDeletion {
		return result, nil
	}
	request, err := db.PendingUserDeletion(ctx, database, user.ID)
	if err != nil {
		return nil, err
	}
	result.RequestedAt = &request.RequestedAt
	result.ScheduledAt = &request.ScheduledAt
	result.CanCancel = now.Before(request.ScheduledAt)
	return result, nil
}
