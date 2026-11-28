package llm

import (
	"context"
	"fmt"
	"strings"
)

// Groq uses the OpenAI chat format.
type groq struct {
	http    *httpDoer
	apiKey  string
	model   string
	baseURL string
}

func newGroq(cfg Config, h *httpDoer) *groq {
	model := cfg.Model
	if model == "" {
		model = DefaultGroqModel
	}
	base := cfg.BaseURL
	if base == "" {
		base = "https://api.groq.com/openai/v1"
	}
	return &groq{http: h, apiKey: cfg.APIKey, model: model, baseURL: strings.TrimRight(base, "/")}
}

type groqMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type groqRequest struct {
	Model          string            `json:"model"`
	Messages       []groqMessage     `json:"messages"`
	Temperature    float64           `json:"temperature"`
	MaxTokens      int               `json:"max_tokens"`
	ResponseFormat map[string]string `json:"response_format,omitempty"`
}

type groqResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

func (g *groq) Chat(ctx context.Context, req Request) (string, error) {
	body := groqRequest{Model: g.model, Temperature: req.Temperature, MaxTokens: req.MaxTokens}
	if body.MaxTokens <= 0 {
		body.MaxTokens = 2048
	}
	for _, m := range req.Messages {
		body.Messages = append(body.Messages, groqMessage{Role: string(m.Role), Content: m.Content})
	}
	if len(body.Messages) == 0 {
		return "", fmt.Errorf("llm: request has no messages")
	}
	if req.JSON {
		// JSON mode needs the word "JSON" somewhere in the prompt. Our prompts have it.
		body.ResponseFormat = map[string]string{"type": "json_object"}
	}

	var resp groqResponse
	headers := map[string]string{"Authorization": "Bearer " + g.apiKey}
	if err := g.http.postJSON(ctx, g.baseURL+"/chat/completions", headers, body, &resp); err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", ErrEmptyReply
	}
	out := strings.TrimSpace(resp.Choices[0].Message.Content)
	if out == "" {
		if fr := resp.Choices[0].FinishReason; fr != "" && fr != "stop" {
			return "", fmt.Errorf("%w (finish reason: %s)", ErrEmptyReply, fr)
		}
		return "", ErrEmptyReply
	}
	return out, nil
}
