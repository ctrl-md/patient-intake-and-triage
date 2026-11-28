package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// httpDoer sends one JSON POST, with a timeout per try and retries.
type httpDoer struct {
	client     *http.Client
	timeout    time.Duration
	maxRetries int
	backoff    time.Duration
	// sleep waits between tries. Tests replace it so they run fast.
	sleep func(ctx context.Context, d time.Duration) error
}

func newHTTPDoer(timeout time.Duration, maxRetries int) *httpDoer {
	return &httpDoer{
		client:     &http.Client{},
		timeout:    timeout,
		maxRetries: maxRetries,
		backoff:    500 * time.Millisecond,
		sleep: func(ctx context.Context, d time.Duration) error {
			t := time.NewTimer(d)
			defer t.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-t.C:
				return nil
			}
		},
	}
}

const maxErrorBody = 300

// postJSON sends body as JSON and decodes a 2xx answer into out.
// It retries on network errors, 429 and 5xx. It never retries other 4xx.
func (d *httpDoer) postJSON(ctx context.Context, url string, headers map[string]string, body, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("llm: encode request: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt <= d.maxRetries; attempt++ {
		if attempt > 0 {
			wait := d.backoff << (attempt - 1)
			if ra := retryAfter(lastErr); ra > 0 {
				wait = ra
			}
			if err := d.sleep(ctx, wait); err != nil {
				return err
			}
		}

		lastErr = d.once(ctx, url, headers, payload, out)
		if lastErr == nil {
			return nil
		}
		if ctx.Err() != nil { // the caller gave up, so do not try again
			return lastErr
		}
		var apiErr *apiErrorWithRetry
		if errors.As(lastErr, &apiErr) && !apiErr.retryable() {
			return apiErr.APIError
		}
		var plain *APIError
		if errors.As(lastErr, &plain) && !plain.retryable() {
			return lastErr
		}
	}

	var withRetry *apiErrorWithRetry
	if errors.As(lastErr, &withRetry) {
		return withRetry.APIError
	}
	return lastErr
}

// apiErrorWithRetry carries the Retry-After hint from the provider.
type apiErrorWithRetry struct {
	*APIError
	after time.Duration
}

func (e *apiErrorWithRetry) Unwrap() error { return e.APIError }

func retryAfter(err error) time.Duration {
	var e *apiErrorWithRetry
	if errors.As(err, &e) {
		return e.after
	}
	return 0
}

func (d *httpDoer) once(ctx context.Context, url string, headers map[string]string, payload []byte, out any) error {
	ctx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("llm: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("llm: request failed: %w", err) // network or timeout: retryable
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return fmt.Errorf("llm: read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		apiErr := &APIError{Status: resp.StatusCode, Message: errorText(raw)}
		return &apiErrorWithRetry{APIError: apiErr, after: parseRetryAfter(resp.Header.Get("Retry-After"))}
	}

	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("llm: decode response: %w", err)
	}
	return nil
}

// errorText pulls the message out of {"error":{"message":"..."}} when it can.
// It never includes request headers, so the API key cannot leak into logs.
func errorText(raw []byte) string {
	var shaped struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(raw, &shaped) == nil && shaped.Error.Message != "" {
		return truncate(shaped.Error.Message, maxErrorBody)
	}
	return truncate(string(raw), maxErrorBody)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// parseRetryAfter reads a Retry-After in seconds. It is capped at 5s so one
// slow provider cannot hold a patient's request for long.
func parseRetryAfter(v string) time.Duration {
	secs, err := strconv.Atoi(v)
	if err != nil || secs <= 0 {
		return 0
	}
	if secs > 5 {
		secs = 5
	}
	return time.Duration(secs) * time.Second
}
