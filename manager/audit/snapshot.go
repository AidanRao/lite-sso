package audit

import (
	"encoding/json"
	"net"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"

	"sso-server/model"
	"sso-server/util/mask"
)

const maxEventBytes = 4096

func snapshot(event model.AuditLog) (model.AuditLog, bool) {
	if _, err := uuid.Parse(event.ID); err != nil {
		return model.AuditLog{}, false
	}
	if event.OccurredAt.IsZero() {
		return model.AuditLog{}, false
	}
	if event.Outcome != "success" && event.Outcome != "failure" && event.Outcome != "denied" {
		return model.AuditLog{}, false
	}
	event.UserID = optional(event.UserID, 36)
	if event.UserID == nil {
		event.ActorType = "anonymous"
	} else if event.ActorType != "admin" {
		event.ActorType = "user"
	}
	event.ClientID = optional(event.ClientID, 64)
	event.Action = cleanText(event.Action, 64)
	event.TargetType = cleanText(event.TargetType, 32)
	event.TargetID = cleanText(event.TargetID, 128)
	event.Method = cleanText(event.Method, 16)
	event.Route = cleanText(event.Route, 160)
	event.ReasonCode = cleanText(event.ReasonCode, 64)
	event.DeviceID = cleanText(event.DeviceID, 64)
	event.DeviceLabel = cleanText(event.DeviceLabel, 32)
	if ip := net.ParseIP(event.IP); ip != nil {
		event.IP = ip.String()
	} else {
		event.IP = ""
	}
	if len(event.SessionHash) != 64 || strings.Trim(event.SessionHash, "0123456789abcdef") != "" {
		event.SessionHash = ""
	}
	if event.DurationMS < 0 {
		event.DurationMS = 0
	}
	event.CreatedAt = time.Time{}
	var details model.AuditDetails
	if event.Details != "" && json.Unmarshal([]byte(event.Details), &details) != nil {
		return model.AuditLog{}, false
	}
	details.ChangedFields = allowedValues(details.ChangedFields, "username", "avatar", "password", "email", "verified", "is_primary", "name", "provider", "passkey", "session", "logo", "client_secret", "redirect_uri", "logout_uri", "homepage_url", "is_active", "description")
	details.CompletedSteps = allowedValues(details.CompletedSteps, "user_created", "session_created", "password_updated", "sessions_revoked", "email_created", "verification_sent", "email_verified", "binding_prepared", "binding_created", "authorization_code_issued")
	details.AuthMethod = allowedValue(details.AuthMethod, "password", "email_otp", "qr_code", "github", "feishu", "passkey")
	details.Provider = allowedValue(details.Provider, "github", "feishu")
	if details.EmailMasked != "" {
		details.EmailMasked = cleanText(mask.Email(details.EmailMasked), 128)
	}
	data, err := json.Marshal(details)
	if err != nil {
		return model.AuditLog{}, false
	}
	event.Details = string(data)
	// Reserve space for the insertion timestamp added by the writer.
	encoded, err := json.Marshal(event)
	return event, err == nil && len(encoded) <= maxEventBytes-32
}

func optional(value *string, limit int) *string {
	if value == nil {
		return nil
	}
	copy := cleanText(*value, limit)
	if copy == "" {
		return nil
	}
	return &copy
}

func cleanText(value string, limit int) string {
	value = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == utf8.RuneError {
			return -1
		}
		return r
	}, value)
	if len(value) > limit {
		value = value[:limit]
		for !utf8.ValidString(value) {
			value = value[:len(value)-1]
		}
	}
	return value
}

func allowedValue(value string, allowed ...string) string {
	for _, candidate := range allowed {
		if value == candidate {
			return value
		}
	}
	return ""
}

func allowedValues(values []string, allowed ...string) []string {
	var result []string
	seen := make(map[string]bool)
	for _, value := range values {
		if !seen[value] && allowedValue(value, allowed...) != "" {
			result = append(result, value)
			seen[value] = true
		}
	}
	return result
}
