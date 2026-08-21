package auth

import (
	"context"
	"strings"
	"time"

	"sso-server/common"
	"sso-server/dal/db"
	"sso-server/dto"
)

const deviceRevokeReason = "user_device_revoked"

// ListLoginDevices returns active sessions grouped by the browser device ID.
func (s *AuthService) ListLoginDevices(ctx context.Context, userID string, currentSessionID string) ([]dto.LoginDeviceResponse, error) {
	userID = strings.TrimSpace(userID)
	currentSessionID = strings.TrimSpace(currentSessionID)
	if userID == "" || currentSessionID == "" {
		return nil, common.ErrSessionRevoked
	}

	now := time.Now()
	repository := db.NewUserSessionRepository(s.db)
	currentSession, err := repository.FindActive(ctx, currentSessionID, now)
	if err != nil || currentSession.UserID != userID {
		return nil, common.ErrSessionRevoked
	}

	sessions, err := repository.ListActiveByUserID(ctx, userID, now)
	if err != nil {
		return nil, err
	}

	devices := make([]dto.LoginDeviceResponse, 0, len(sessions))
	deviceIndexes := make(map[string]int, len(sessions))
	for _, session := range sessions {
		deviceID := strings.TrimSpace(session.DeviceID)
		if deviceID == "" {
			continue
		}
		if index, exists := deviceIndexes[deviceID]; exists {
			if session.ID == currentSessionID {
				devices[index].Current = true
			}
			continue
		}

		deviceIndexes[deviceID] = len(devices)
		devices = append(devices, dto.LoginDeviceResponse{
			DeviceID:   deviceID,
			UserAgent:  session.UserAgent,
			IP:         session.IP,
			AuthMethod: session.AuthMethod,
			CreatedAt:  session.CreatedAt,
			LastSeenAt: session.LastSeenAt,
			ExpiresAt:  session.ExpiresAt,
			Current:    session.ID == currentSessionID,
		})
	}

	for index, device := range devices {
		if device.Current && index > 0 {
			copy(devices[1:index+1], devices[0:index])
			devices[0] = device
			break
		}
	}
	return devices, nil
}

// RevokeLoginDevice revokes all active sessions for another user device.
func (s *AuthService) RevokeLoginDevice(ctx context.Context, userID string, currentSessionID string, deviceID string) error {
	userID = strings.TrimSpace(userID)
	currentSessionID = strings.TrimSpace(currentSessionID)
	deviceID = strings.TrimSpace(deviceID)
	if userID == "" || currentSessionID == "" {
		return common.ErrSessionRevoked
	}
	if deviceID == "" {
		return common.ErrDeviceNotFound
	}

	now := time.Now()
	repository := db.NewUserSessionRepository(s.db)
	currentSession, err := repository.FindActive(ctx, currentSessionID, now)
	if err != nil || currentSession.UserID != userID {
		return common.ErrSessionRevoked
	}
	if currentSession.DeviceID == deviceID {
		return common.ErrCurrentDevice
	}

	rowsAffected, err := repository.RevokeDevice(ctx, userID, deviceID, deviceRevokeReason, now)
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return common.ErrDeviceNotFound
	}
	return nil
}
