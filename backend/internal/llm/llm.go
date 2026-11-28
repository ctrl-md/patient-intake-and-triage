// Package llm talks to a hosted language model (Gemini or Groq) over plain HTTP.
// It has timeouts, retries, and a fake for tests. No SDKs, no extra packages.
package llm

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

type Message struct {
	Role    Role
	Content string
}

// Request is one chat call. Put instructions in a RoleSystem message.
type Request struct {
	Messages    []Message
	Temperature float64 // 0 is allowed and means "as steady as possible"
	MaxTokens   int     // 0 means 2048
	JSON        bool    // ask for a JSON object as the reply
}

// ErrEmptyReply means the model answered with no text (often a safety block).
var ErrEmptyReply = errors.New("llm: the model returned no text")

// APIError is an error answer from the provider.
type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("llm: provider returned %d: %s", e.Status, e.Message)
}

// retryable says whether trying again could help.
func (e *APIError) retryable() bool {
	switch e.Status {
	case 429, 500, 502, 503, 504:
		return true
	}
	return false
}

// Config chooses and sets up a provider.
type Config struct {
	Provider   string        // "gemini" or "groq"
	APIKey     string        //
	Model      string        // empty = a default for the provider
	Timeout    time.Duration // per attempt. 0 means 20s
	MaxRetries int           // extra tries after the first. 0 means 2. Use -1 for none
	BaseURL    string        // only for tests
}

// Default models. Model names change often, so set LLM_MODEL if one is retired.
const (
	DefaultGeminiModel = "gemini-2.5-flash"
	DefaultGroqModel   = "llama-3.3-70b-versatile"
)

// New makes a client for the chosen provider.
func New(cfg Config) (Client, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, errors.New("llm: API key is empty")
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 20 * time.Second
	}
	switch {
	case cfg.MaxRetries == 0:
		cfg.MaxRetries = 2
	case cfg.MaxRetries < 0:
		cfg.MaxRetries = 0
	}

	h := newHTTPDoer(cfg.Timeout, cfg.MaxRetries)

	switch strings.ToLower(strings.TrimSpace(cfg.Provider)) {
	case "gemini":
		return newGemini(cfg, h), nil
	case "groq":
		return newGroq(cfg, h), nil
	default:
		return nil, fmt.Errorf("llm: unknown provider %q (use gemini or groq)", cfg.Provider)
	}
}
