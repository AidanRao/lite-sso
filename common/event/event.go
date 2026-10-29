// Package event defines business facts shared by producers and subscribers.
package event

import (
	"time"
)

// LoginSucceededType identifies committed authentication events.
const LoginSucceededType = "auth.login.succeeded"

// Event exposes only the routing and diagnostic identity of a business fact.
type Event interface {
	EventType() string
	EventID() string
}

// Publisher accepts events without waiting for subscribers.
type Publisher interface {
	TryPublish(Event) bool
}

// LoginSucceeded describes a committed login without authentication credentials.
type LoginSucceeded struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	SessionID  string    `json:"session_id"`
	DeviceID   string    `json:"device_id"`
	OccurredAt time.Time `json:"occurred_at"`
	AuthMethod string    `json:"auth_method"`
	IP         string    `json:"ip"`
	Location   string    `json:"location"`
	UserAgent  string    `json:"user_agent"`
	NewDevice  bool      `json:"new_device"`
}

func (LoginSucceeded) EventType() string { return LoginSucceededType }

func (e LoginSucceeded) EventID() string { return e.ID }
