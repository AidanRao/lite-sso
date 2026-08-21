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

func TestClient_Send_NetworkFailure(t *testing.T) {
	client, err := NewClient(Config{URL: "https://message.example.com", APIKey: "api-key", SenderKey: "noreply"})
	require.NoError(t, err)
	client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return nil, errors.New("network unavailable")
	})

	err = client.Send(context.Background(), "user@example.com", "template", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "network unavailable")
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
