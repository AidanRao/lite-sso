// Package messagecenter provides access to the Message Center sending API.
package messagecenter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	messagesPath        = "/v1/messages"
	defaultHTTPTimeout  = 10 * time.Second
	maximumErrorBodyLen = 4096
	emailChannel         = "EMAIL"
)

// Config contains the connection settings required by Message Center.
type Config struct {
	URL       string
	APIKey    string
	SenderKey string
}

// Client sends messages through Message Center.
type Client struct {
	endpoint   string
	apiKey     string
	senderKey  string
	httpClient *http.Client
}

type sendRequest struct {
	SenderKey   string            `json:"senderKey"`
	TemplateKey string            `json:"templateKey"`
	Target      string            `json:"target"`
	Variables   map[string]string `json:"variables"`
	Channel     string            `json:"channel"`
}

// NewClient validates the configuration and creates a Message Center client.
func NewClient(cfg Config) (*Client, error) {
	baseURL := strings.TrimSpace(cfg.URL)
	apiKey := strings.TrimSpace(cfg.APIKey)
	senderKey := strings.TrimSpace(cfg.SenderKey)
	if baseURL == "" {
		return nil, errors.New("message center URL is required")
	}
	if apiKey == "" {
		return nil, errors.New("message center API key is required")
	}
	if senderKey == "" {
		return nil, errors.New("message center sender key is required")
	}

	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse message center URL: %w", err)
	}
	if (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || parsedURL.Host == "" {
		return nil, errors.New("message center URL must be an absolute HTTP or HTTPS URL")
	}
	if parsedURL.RawQuery != "" || parsedURL.Fragment != "" {
		return nil, errors.New("message center URL must not include a query or fragment")
	}
	parsedURL.Path = strings.TrimRight(parsedURL.Path, "/") + messagesPath

	return &Client{
		endpoint:  parsedURL.String(),
		apiKey:    apiKey,
		senderKey: senderKey,
		httpClient: &http.Client{
			Timeout: defaultHTTPTimeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
}

// Send submits one templated message for asynchronous delivery.
func (c *Client) Send(ctx context.Context, target string, templateKey string, variables map[string]string) error {
	payload, err := json.Marshal(sendRequest{
		SenderKey:   c.senderKey,
		TemplateKey: templateKey,
		Target:      target,
		Variables:   variables,
		Channel:     emailChannel,
	})
	if err != nil {
		return fmt.Errorf("marshal message center request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create message center request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", uuid.NewString())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send message center request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusAccepted {
		return nil
	}

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, maximumErrorBodyLen))
	if readErr != nil {
		return fmt.Errorf("message center returned status %d and response could not be read: %w", resp.StatusCode, readErr)
	}
	return fmt.Errorf("message center returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
}
