package llm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func fast(c Client) {
	var h *httpDoer
	switch v := c.(type) {
	case *gemini:
		h = v.http
	case *groq:
		h = v.http
	}
	h.sleep = func(context.Context, time.Duration) error { return nil }
}

func newTestGemini(t *testing.T, url string, retries int) Client {
	t.Helper()
	c, err := New(Config{Provider: "gemini", APIKey: "k", BaseURL: url, MaxRetries: retries, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	fast(c)
	return c
}

func TestNewValidation(t *testing.T) {
	if _, err := New(Config{Provider: "gemini"}); err == nil {
		t.Fatal("empty key should fail")
	}
	if _, err := New(Config{Provider: "nope", APIKey: "k"}); err == nil {
		t.Fatal("unknown provider should fail")
	}
}

func TestGeminiRequestAndReply(t *testing.T) {
	var gotKey, gotPath string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey, gotPath = r.Header.Get("x-goog-api-key"), r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"hmm","thought":true},{"text":" hello "}]}}]}`))
	}))
	defer srv.Close()
	c := newTestGemini(t, srv.URL, 0)
	out, err := c.Chat(context.Background(), Request{JSON: true, Messages: []Message{
		{RoleSystem, "be kind"}, {RoleUser, "hi"}, {RoleAssistant, "yo"}, {RoleUser, "again"}}})
	if err != nil {
		t.Fatal(err)
	}
	if out != "hello" {
		t.Fatalf("got %q", out)
	}
	if gotKey != "k" || !strings.HasSuffix(gotPath, ":generateContent") {
		t.Fatalf("key=%q path=%q", gotKey, gotPath)
	}
	if body["systemInstruction"] == nil {
		t.Fatal("system instruction missing")
	}
	contents := body["contents"].([]any)
	if len(contents) != 3 || contents[1].(map[string]any)["role"] != "model" {
		t.Fatalf("bad contents: %v", contents)
	}
	if body["generationConfig"].(map[string]any)["responseMimeType"] != "application/json" {
		t.Fatal("json mime missing")
	}
}

func TestGeminiBlockedAndEmpty(t *testing.T) {
	for _, resp := range []string{
		`{"candidates":[],"promptFeedback":{"blockReason":"SAFETY"}}`,
		`{"candidates":[{"content":{"parts":[]},"finishReason":"SAFETY"}]}`,
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(resp)) }))
		c := newTestGemini(t, srv.URL, 0)
		_, err := c.Chat(context.Background(), Request{Messages: []Message{{RoleUser, "x"}}})
		srv.Close()
		if !errors.Is(err, ErrEmptyReply) {
			t.Fatalf("want ErrEmptyReply, got %v", err)
		}
	}
}

func TestRetriesOn503ThenSucceeds(t *testing.T) {
	var n int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&n, 1) < 3 {
			w.WriteHeader(503)
			_, _ = w.Write([]byte(`{"error":{"message":"busy"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"ok"}]}}]}`))
	}))
	defer srv.Close()
	c := newTestGemini(t, srv.URL, 2)
	out, err := c.Chat(context.Background(), Request{Messages: []Message{{RoleUser, "x"}}})
	if err != nil || out != "ok" || n != 3 {
		t.Fatalf("out=%q err=%v calls=%d", out, err, n)
	}
}

func TestNoRetryOn400(t *testing.T) {
	var n int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&n, 1)
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"error":{"message":"bad model"}}`))
	}))
	defer srv.Close()
	c := newTestGemini(t, srv.URL, 3)
	_, err := c.Chat(context.Background(), Request{Messages: []Message{{RoleUser, "x"}}})
	var ae *APIError
	if !errors.As(err, &ae) || ae.Status != 400 || ae.Message != "bad model" {
		t.Fatalf("err=%v", err)
	}
	if n != 1 {
		t.Fatalf("retried %d times", n)
	}
}

func TestGivesUpAfterRetries(t *testing.T) {
	var n int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&n, 1)
		w.WriteHeader(429)
	}))
	defer srv.Close()
	c := newTestGemini(t, srv.URL, 2)
	_, err := c.Chat(context.Background(), Request{Messages: []Message{{RoleUser, "x"}}})
	var ae *APIError
	if !errors.As(err, &ae) || ae.Status != 429 || n != 3 {
		t.Fatalf("err=%v calls=%d", err, n)
	}
}

func TestTimeoutPerAttempt(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	}))
	defer srv.Close()
	c, _ := New(Config{Provider: "gemini", APIKey: "k", BaseURL: srv.URL, MaxRetries: -1, Timeout: 50 * time.Millisecond})
	start := time.Now()
	_, err := c.Chat(context.Background(), Request{Messages: []Message{{RoleUser, "x"}}})
	if err == nil || time.Since(start) > time.Second {
		t.Fatalf("expected quick timeout, err=%v after %v", err, time.Since(start))
	}
}

func TestCallerCancelStopsRetries(t *testing.T) {
	var n int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&n, 1)
		w.WriteHeader(503)
	}))
	defer srv.Close()
	c := newTestGemini(t, srv.URL, 5)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Chat(ctx, Request{Messages: []Message{{RoleUser, "x"}}}); err == nil {
		t.Fatal("expected error")
	}
	if n > 1 {
		t.Fatalf("kept retrying: %d", n)
	}
}

func TestGroqRequestAndReply(t *testing.T) {
	var auth string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":" {\"a\":1} "},"finish_reason":"stop"}]}`))
	}))
	defer srv.Close()
	c, _ := New(Config{Provider: "groq", APIKey: "gk", BaseURL: srv.URL})
	fast(c)
	out, err := c.Chat(context.Background(), Request{JSON: true, Messages: []Message{{RoleSystem, "s"}, {RoleUser, "u"}}})
	if err != nil || out != `{"a":1}` {
		t.Fatalf("out=%q err=%v", out, err)
	}
	if auth != "Bearer gk" || body["model"] != DefaultGroqModel {
		t.Fatalf("auth=%q model=%v", auth, body["model"])
	}
	if body["response_format"].(map[string]any)["type"] != "json_object" {
		t.Fatal("json mode missing")
	}
}

func TestErrorDoesNotLeakKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(401) }))
	defer srv.Close()
	c := newTestGemini(t, srv.URL, 0)
	_, err := c.Chat(context.Background(), Request{Messages: []Message{{RoleUser, "x"}}})
	if err == nil || strings.Contains(err.Error(), "k\"") {
		t.Fatalf("err=%v", err)
	}
}

func TestFake(t *testing.T) {
	f := NewFake(Say("a"), Fail(errors.New("boom")))
	if out, _ := f.Chat(context.Background(), Request{}); out != "a" {
		t.Fatal("first")
	}
	if _, err := f.Chat(context.Background(), Request{}); err == nil {
		t.Fatal("second")
	}
	if _, err := f.Chat(context.Background(), Request{}); err == nil {
		t.Fatal("exhausted")
	}
	if len(f.Calls()) != 3 {
		t.Fatal("calls")
	}
}
