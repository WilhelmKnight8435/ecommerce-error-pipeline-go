package errorpipeline

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestCaptureRetries429WithSameIdempotencyKey(t *testing.T) {
	var calls int
	var keys []string
	client := &Client{
		BaseURL: "https://example.test",
		APIKey:  "test-key",
		HTTP: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			calls++
			keys = append(keys, req.Header.Get("Idempotency-Key"))
			if req.Method != http.MethodPost || req.URL.Path != "/v1/errors/capture" {
				t.Fatalf("request = %s %s", req.Method, req.URL.Path)
			}
			if calls == 1 {
				return &http.Response{StatusCode: 429, Header: http.Header{"Retry-After": []string{"2"}}, Body: io.NopCloser(strings.NewReader(`{"ok":false,"error":{"message":"rate limited"}}`))}, nil
			}
			return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"ok":true,"data":{"event_id":"evt-7"},"error":null,"metadata":{}}`))}, nil
		})},
		Sleep: func(_ context.Context, delay time.Duration) error {
			if delay != 2*time.Second {
				t.Fatalf("delay = %s", delay)
			}
			return nil
		},
	}
	_, err := client.Capture(context.Background(), map[string]any{"message": "payment rejected"}, "capture-ord-1042")
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || keys[0] != "capture-ord-1042" || keys[1] != keys[0] {
		t.Fatalf("calls=%d keys=%v", calls, keys)
	}
}
