package messagecenter

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Client_SendEmail_CustomSenderJSONAndIdempotency(t *testing.T) {
	client, err := NewClient(Config{URL: "https://message.example.com", APIKey: "existing-otp-key", SenderKey: "default"})
	require.NoError(t, err)
	var keys []string
	client.httpClient.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		assert.Equal(t, "Bearer existing-otp-key", request.Header.Get("Authorization"))
		keys = append(keys, request.Header.Get("Idempotency-Key"))
		body, err := io.ReadAll(request.Body)
		require.NoError(t, err)
		assert.JSONEq(t, `{"senderKey":"chosen","templateKey":"notice","target":"recipient@example.com","variables":{"count":2,"flag":true,"nested":[null,{"text":"通知"}]},"channel":"EMAIL"}`, string(body))
		return &http.Response{StatusCode: http.StatusAccepted, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
	})
	message := EmailMessage{SenderKey: "chosen", TemplateKey: "notice", Target: "recipient@example.com", Variables: map[string]any{"count": 2, "flag": true, "nested": []any{nil, map[string]any{"text": "通知"}}}, IdempotencyKey: "sso-admin:1:member:fixed"}
	require.NoError(t, client.SendEmail(context.Background(), message))
	require.NoError(t, client.SendEmail(context.Background(), message))
	assert.Equal(t, []string{message.IdempotencyKey, message.IdempotencyKey}, keys)
}

func Test_Client_SendEmail_OutcomeClassification(t *testing.T) {
	for _, scenario := range []struct {
		name    string
		status  int
		err     error
		unknown bool
	}{
		{name: "rejected", status: 400},
		{name: "server failure", status: 502, unknown: true},
		{name: "transport", err: errors.New("connection lost"), unknown: true},
		{name: "deadline", err: context.DeadlineExceeded, unknown: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			client, err := NewClient(Config{URL: "https://message.example.com", APIKey: "key", SenderKey: "default"})
			require.NoError(t, err)
			client.httpClient.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
				if scenario.err != nil {
					return nil, scenario.err
				}
				return &http.Response{StatusCode: scenario.status, Body: io.NopCloser(strings.NewReader(`{"message":"private upstream details"}`)), Header: make(http.Header)}, nil
			})
			err = client.SendEmail(context.Background(), EmailMessage{SenderKey: "chosen", TemplateKey: "notice", Target: "recipient@example.com", Variables: map[string]any{}, IdempotencyKey: "fixed"})
			var submission *SubmissionError
			require.ErrorAs(t, err, &submission)
			assert.Equal(t, scenario.unknown, submission.Unknown)
			if scenario.err != nil {
				assert.ErrorIs(t, err, scenario.err)
			}
		})
	}
}

func TestClient_Send_Accepted(t *testing.T) {
	var received sendRequest
	client, err := NewClient(Config{
		URL:       "https://message.example.com/base/",
		APIKey:    "api-key",
		SenderKey: "noreply",
	})
	require.NoError(t, err)
	client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/base/v1/messages", r.URL.Path)
		assert.Equal(t, "Bearer api-key", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		_, err := uuid.Parse(r.Header.Get("Idempotency-Key"))
		assert.NoError(t, err)
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(body, &received))
		assert.JSONEq(t, `{"senderKey":"noreply","templateKey":"sso-verify-code-email","target":"user@example.com","variables":{"code":"123456"},"channel":"EMAIL"}`, string(body))
		return &http.Response{
			StatusCode: http.StatusAccepted,
			Body:       io.NopCloser(strings.NewReader(`{"success":true}`)),
			Header:     make(http.Header),
		}, nil
	})

	err = client.Send(context.Background(), "user@example.com", "sso-verify-code-email", map[string]string{"code": "123456"})
	require.NoError(t, err)
	assert.Equal(t, sendRequest{
		SenderKey:   "noreply",
		TemplateKey: "sso-verify-code-email",
		Target:      "user@example.com",
		Variables:   map[string]string{"code": "123456"},
		Channel:     emailChannel,
	}, received)
}

func TestClient_Send_NonAcceptedResponse(t *testing.T) {
	client, err := NewClient(Config{URL: "https://message.example.com", APIKey: "api-key", SenderKey: "noreply"})
	require.NoError(t, err)
	client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Body:       io.NopCloser(strings.NewReader(`{"message":"invalid request"}`)),
			Header:     make(http.Header),
		}, nil
	})

	err = client.Send(context.Background(), "user@example.com", "template", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status 400")
	assert.Contains(t, err.Error(), "invalid request")
}

func TestClient_Send_Timeout(t *testing.T) {
	client, err := NewClient(Config{URL: "https://message.example.com", APIKey: "api-key", SenderKey: "noreply"})
	require.NoError(t, err)
	client.httpClient.Timeout = 10 * time.Millisecond
	client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		<-r.Context().Done()
		return nil, r.Context().Err()
	})

	err = client.Send(context.Background(), "user@example.com", "template", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "send message center request")
}

func TestNewClient_InvalidConfig(t *testing.T) {
	tests := []struct {
		name      string
		config    Config
		errorText string
	}{
		{name: "missing URL", config: Config{APIKey: "key", SenderKey: "sender"}, errorText: "URL is required"},
		{name: "missing API key", config: Config{URL: "https://message.example.com", SenderKey: "sender"}, errorText: "API key is required"},
		{name: "missing sender key", config: Config{URL: "https://message.example.com", APIKey: "key"}, errorText: "sender key is required"},
		{name: "relative URL", config: Config{URL: "/message", APIKey: "key", SenderKey: "sender"}, errorText: "absolute HTTP or HTTPS"},
		{name: "URL with query", config: Config{URL: "https://message.example.com?tenant=one", APIKey: "key", SenderKey: "sender"}, errorText: "query or fragment"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.config)
			assert.Nil(t, client)
			require.Error(t, err)
			assert.True(t, strings.Contains(err.Error(), tt.errorText), err.Error())
		})
	}
}

type roundTripFunc func(r *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
