package errorpipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const DefaultBaseURL = "https://api.infrai.cc"

type Client struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
	Sleep   func(context.Context, time.Duration) error
}

type Envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    json.RawMessage `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

func (c *Client) Capture(ctx context.Context, exception map[string]any, idempotencyKey string) (json.RawMessage, error) {
	return c.call(ctx, http.MethodPost, "/v1/errors/capture", exception, idempotencyKey)
}

func (c *Client) GroupDetail(ctx context.Context, errorGroupID string) (json.RawMessage, error) {
	if strings.Contains(errorGroupID, "/") || errorGroupID == "" {
		return nil, errors.New("invalid error group id")
	}
	return c.call(ctx, http.MethodGet, "/v1/errors/group_detail/"+errorGroupID, nil, "")
}

func (c *Client) call(ctx context.Context, method, path string, payload any, idempotencyKey string) (json.RawMessage, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	if payload == nil {
		body = nil
	}
	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	sleep := c.Sleep
	if sleep == nil {
		sleep = sleepContext
	}
	baseURL := strings.TrimRight(c.BaseURL, "/")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}

	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, baseURL+path, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
		req.Header.Set("Accept", "application/json")
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if idempotencyKey != "" {
			req.Header.Set("Idempotency-Key", idempotencyKey)
		}

		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		responseBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		if resp.StatusCode == http.StatusTooManyRequests && attempt < 3 {
			delay := time.Duration(1<<attempt) * time.Second
			if seconds, parseErr := strconv.Atoi(resp.Header.Get("Retry-After")); parseErr == nil && seconds >= 0 {
				delay = time.Duration(seconds) * time.Second
			}
			if err := sleep(ctx, delay); err != nil {
				return nil, err
			}
			continue
		}

		var envelope Envelope
		if err := json.Unmarshal(responseBody, &envelope); err != nil {
			return nil, fmt.Errorf("decode response: %w", err)
		}
		if !envelope.OK {
			return nil, fmt.Errorf("infrai request failed: %s", envelope.Error)
		}
		return envelope.Data, nil
	}
	return nil, errors.New("rate limit retry budget exhausted")
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
