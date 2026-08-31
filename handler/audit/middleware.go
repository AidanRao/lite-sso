// Package audit collects safe HTTP operation metadata without inspecting bodies.
package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"sso-server/conf"
	"sso-server/model"
	"sso-server/util/mask"
	"sso-server/util/useragent"
)

const contextKey = "operation_audit"

// Sink accepts a sanitized copy synchronously and persists it asynchronously.
type Sink interface{ TryRecord(model.AuditLog) bool }

// Rule declares an audited endpoint and its non-secret resource parameter.
type Rule struct {
	Method      string
	Route       string
	Action      string
	TargetType  string
	TargetParam string
	AuthMethod  string
}

type operation struct {
	event     model.AuditLog
	details   model.AuditDetails
	sessionID string
}

// Middleware surrounds recovery and authorization so rejected requests are recorded too.
func Middleware(sink Sink, cfg *conf.Config, source func(*http.Request) (string, string)) gin.HandlerFunc {
	rules := make(map[string]Rule)
	for _, rule := range Rules() {
		rules[rule.Method+" "+rule.Route] = rule
	}
	return func(c *gin.Context) {
		rule, ok := rules[c.Request.Method+" "+c.FullPath()]
		if !ok {
			c.Next()
			return
		}
		started := time.Now()
		ip, deviceID := source(c.Request)
		op := &operation{event: model.AuditLog{
			ID: uuid.NewString(), OccurredAt: started.UTC(), Action: rule.Action,
			TargetType: rule.TargetType, Method: c.Request.Method, Route: c.FullPath(),
			IP: ip, DeviceID: deviceID,
			DeviceLabel: useragent.Parse(c.Request.UserAgent()).Label,
		}, details: model.AuditDetails{AuthMethod: rule.AuthMethod}}
		if rule.TargetParam != "" {
			op.event.TargetID = c.Param(rule.TargetParam)
		}
		c.Set(contextKey, op)
		Provider(c, c.Param("provider"))
		c.Next()
		if c.IsAborted() && len(c.Errors) > 0 {
			Failure(c, "REQUEST_ABORTED")
		}
		if op.event.UserID == nil {
			Actor(c, c.GetString("user_id"), c.GetString("session_id"))
		}
		if op.event.TargetType == "user" && op.event.TargetID == "" && op.event.UserID != nil {
			op.event.TargetID = *op.event.UserID
		}
		op.event.ActorType = "anonymous"
		if op.event.UserID != nil {
			op.event.ActorType = "user"
			if cfg.IsAdminUser(*op.event.UserID) {
				op.event.ActorType = "admin"
			}
		}
		if op.sessionID != "" {
			digest := sha256.Sum256([]byte(op.sessionID))
			op.event.SessionHash = hex.EncodeToString(digest[:])
		}
		op.event.HTTPStatus = c.Writer.Status()
		op.event.DurationMS = time.Since(started).Milliseconds()
		if op.event.Outcome == "" {
			switch status := op.event.HTTPStatus; {
			case status == http.StatusUnauthorized || status == http.StatusForbidden || status == http.StatusTooManyRequests:
				Denied(c, fmt.Sprintf("HTTP_%d", status))
			case status >= 200 && status < 300:
				Success(c)
			case status >= 300 && status < 400:
				Failure(c, "UNCLASSIFIED_REDIRECT")
			default:
				Failure(c, fmt.Sprintf("HTTP_%d", status))
			}
		}
		if op.event.Outcome == "failure" && (op.event.HTTPStatus == 401 || op.event.HTTPStatus == 403 || op.event.HTTPStatus == 429) {
			op.event.Outcome = "denied"
		}
		data, _ := json.Marshal(op.details)
		op.event.Details = string(data)
		sink.TryRecord(op.event)
	}
}

// Actor attaches a server-verified identity without modifying authorization state.
func Actor(c *gin.Context, userID, sessionID string) {
	if op := current(c); op != nil {
		if userID != "" {
			op.event.UserID = &userID
		}
		if sessionID != "" {
			op.sessionID = sessionID
		}
	}
}

// Device records a device generated during this request without storing cookies.
func Device(c *gin.Context, deviceID string) {
	if op := current(c); op != nil {
		op.event.DeviceID = deviceID
	}
}

// Target attaches a resource identifier, never a pending binding or challenge token.
func Target(c *gin.Context, kind, id string) {
	if op := current(c); op != nil {
		op.event.TargetType, op.event.TargetID = kind, id
	}
}

// Client records a validated application identifier.
func Client(c *gin.Context, clientID string) {
	if op := current(c); op != nil && clientID != "" {
		op.event.ClientID = &clientID
	}
}

// Email records only a masked display value, including for anonymous attempts.
func Email(c *gin.Context, email string) {
	if op := current(c); op != nil && email != "" {
		op.details.EmailMasked = mask.Email(email)
	}
}

// Provider selects only supported provider names and never retains arbitrary parameters.
func Provider(c *gin.Context, provider string) {
	if op := current(c); op != nil && (provider == "github" || provider == "feishu") {
		op.details.Provider = provider
	}
}

// AuthMethod records the authentication method selected by the server.
func AuthMethod(c *gin.Context, method string) {
	if op := current(c); op != nil {
		op.details.AuthMethod = method
	}
}

// Changed records field names only, not their previous or new values.
func Changed(c *gin.Context, fields ...string) {
	if op := current(c); op != nil {
		op.details.ChangedFields = append(op.details.ChangedFields, fields...)
	}
}

// Completed preserves committed work when a later operation step fails.
func Completed(c *gin.Context, steps ...string) {
	if op := current(c); op != nil {
		op.details.CompletedSteps = append(op.details.CompletedSteps, steps...)
	}
}

// BindingCallback distinguishes a verified binding flow from a sign-in callback.
func BindingCallback(c *gin.Context) {
	if op := current(c); op != nil {
		op.event.Action = "user.third_party.callback"
		op.event.TargetType = "provider"
		op.event.TargetID = op.details.Provider
	}
}

// Success marks completion of this endpoint's declared operation, not later flow steps.
func Success(c *gin.Context) { setResult(c, "success", "OK") }

// Failure attaches a constant safe reason code, not an error's raw message.
func Failure(c *gin.Context, reason string) { setResult(c, "failure", reason) }

// Denied records authentication, authorization, or rate-limit rejection.
func Denied(c *gin.Context, reason string) { setResult(c, "denied", reason) }

func current(c *gin.Context) *operation {
	value, exists := c.Get(contextKey)
	if !exists {
		return nil
	}
	op, _ := value.(*operation)
	return op
}

func setResult(c *gin.Context, outcome, reason string) {
	if op := current(c); op != nil {
		op.event.Outcome, op.event.ReasonCode = outcome, reason
	}
}
