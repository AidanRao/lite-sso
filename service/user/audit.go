package user

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"sso-server/dal/db"
	"sso-server/dto"
	"sso-server/model"
)

const auditQueryWindow = 30 * 24 * time.Hour

// ErrAuditQueryInvalid denotes invalid filters without exposing database errors.
var ErrAuditQueryInvalid = errors.New("invalid audit query")

var auditActionPattern = regexp.MustCompile(`^[a-z][a-z0-9_.]{0,63}$`)

type auditCursor struct {
	Time time.Time `json:"time"`
	ID   string    `json:"id"`
}

// ListAuditLogs returns this user's records within the server's rolling 30-day window.
func (s *UserService) ListAuditLogs(ctx context.Context, userID string, request dto.AuditLogQuery) (*dto.AuditLogPage, error) {
	return s.listAuditLogs(ctx, userID, request, time.Now().UTC())
}

func (s *UserService) listAuditLogs(ctx context.Context, userID string, request dto.AuditLogQuery, now time.Time) (*dto.AuditLogPage, error) {
	filter, err := parseAuditQuery(userID, request, now)
	if err != nil {
		return nil, err
	}
	page := &dto.AuditLogPage{Items: []dto.AuditLogResponse{}}
	if !filter.From.Before(filter.To) {
		return page, nil
	}
	records, err := db.NewAuditRepository(s.db).ListForUser(ctx, filter)
	if err != nil {
		return nil, err
	}
	limit := filter.Limit - 1
	if len(records) > limit {
		records = records[:limit]
		last := records[len(records)-1]
		data, err := json.Marshal(auditCursor{Time: last.OccurredAt.UTC(), ID: last.ID})
		if err != nil {
			return nil, err
		}
		page.NextCursor = base64.RawURLEncoding.EncodeToString(data)
	}
	clientIDs := make([]string, 0)
	seenClients := make(map[string]bool)
	for _, record := range records {
		if record.ClientID != nil && *record.ClientID != "" && !seenClients[*record.ClientID] {
			clientIDs = append(clientIDs, *record.ClientID)
			seenClients[*record.ClientID] = true
		}
	}
	clients, err := db.NewOAuthClientRepository(s.db).FindDisplayByClientIDs(ctx, clientIDs)
	if err != nil {
		return nil, err
	}
	applications := make(map[string]*dto.AuditApplicationResponse, len(clients))
	for _, client := range clients {
		applications[client.ClientID] = &dto.AuditApplicationResponse{ClientID: client.ClientID, Name: client.Name, LogoURL: client.LogoURL, HomepageURL: client.HomepageURL}
	}
	for _, record := range records {
		var details model.AuditDetails
		if err := json.Unmarshal([]byte(record.Details), &details); err != nil {
			return nil, err
		}
		var application *dto.AuditApplicationResponse
		if record.ClientID != nil {
			application = applications[*record.ClientID]
		}
		page.Items = append(page.Items, dto.AuditLogResponse{
			ID: record.ID, OccurredAt: record.OccurredAt, Action: record.Action,
			Outcome: record.Outcome, ReasonCode: record.ReasonCode,
			TargetType: record.TargetType, TargetID: record.TargetID, ClientID: record.ClientID,
			Application: application,
			Method:      record.Method, Route: record.Route, HTTPStatus: record.HTTPStatus,
			DurationMS: record.DurationMS, IP: record.IP, DeviceLabel: record.DeviceLabel, Details: details,
		})
	}
	return page, nil
}

func parseAuditQuery(userID string, request dto.AuditLogQuery, now time.Time) (db.AuditLogQuery, error) {
	filter := db.AuditLogQuery{UserID: userID, From: now.Add(-auditQueryWindow), To: now, Search: strings.TrimSpace(request.Q), Outcome: request.Outcome, Limit: 31}
	if userID == "" || !utf8.ValidString(request.Q) || utf8.RuneCountInString(request.Q) > 100 || strings.ContainsRune(request.Q, 0) {
		return filter, ErrAuditQueryInvalid
	}
	if request.Limit != nil {
		if *request.Limit < 1 || *request.Limit > 100 {
			return filter, ErrAuditQueryInvalid
		}
		filter.Limit = *request.Limit + 1
	}
	if request.Outcome != "" && request.Outcome != "success" && request.Outcome != "failure" && request.Outcome != "denied" {
		return filter, ErrAuditQueryInvalid
	}
	if len(request.Actions) > 4160 {
		return filter, ErrAuditQueryInvalid
	}
	if request.Actions != "" {
		actions := strings.Split(request.Actions, ",")
		if len(actions) > 64 {
			return filter, ErrAuditQueryInvalid
		}
		seen := make(map[string]bool)
		for _, action := range actions {
			if !auditActionPattern.MatchString(action) {
				return filter, ErrAuditQueryInvalid
			}
			if !seen[action] {
				filter.Actions = append(filter.Actions, action)
				seen[action] = true
			}
		}
	}
	var from, to time.Time
	var err error
	if request.From != "" {
		from, err = time.Parse(time.RFC3339Nano, request.From)
		if err != nil {
			return filter, ErrAuditQueryInvalid
		}
		if from.After(filter.From) {
			filter.From = from.UTC()
		}
	}
	if request.To != "" {
		to, err = time.Parse(time.RFC3339Nano, request.To)
		if err != nil {
			return filter, ErrAuditQueryInvalid
		}
		if to.Before(filter.To) {
			filter.To = to.UTC()
		}
	}
	if !from.IsZero() && !to.IsZero() && !from.Before(to) {
		return filter, ErrAuditQueryInvalid
	}
	if request.Cursor != "" {
		if len(request.Cursor) > 512 {
			return filter, ErrAuditQueryInvalid
		}
		data, err := base64.RawURLEncoding.DecodeString(request.Cursor)
		if err != nil {
			return filter, ErrAuditQueryInvalid
		}
		var cursor auditCursor
		decoder := json.NewDecoder(strings.NewReader(string(data)))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&cursor) != nil || decoder.Decode(new(any)) != io.EOF || cursor.Time.IsZero() {
			return filter, ErrAuditQueryInvalid
		}
		id, err := uuid.Parse(cursor.ID)
		if err != nil {
			return filter, ErrAuditQueryInvalid
		}
		filter.CursorTime, filter.CursorID = cursor.Time.UTC(), id.String()
	}
	return filter, nil
}
