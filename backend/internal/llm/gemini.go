package llm

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

type gemini struct {
	http    *httpDoer
	apiKey  string
	model   string
	baseURL string
}

func newGemini(cfg Config, h *httpDoer) *gemini {
	model := cfg.Model
	if model == "" {
		model = DefaultGeminiModel
	}
	base := cfg.BaseURL
	if base == "" {
		base = "https://generativelanguage.googleapis.com"
	}
	return &gemini{http: h, apiKey: cfg.APIKey, model: model, baseURL: strings.TrimRight(base, "/")}
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiRequest struct {
	SystemInstruction *geminiContent  `json:"systemInstruction,omitempty"`
	Contents          []geminiContent `json:"contents"`
	GenerationConfig  geminiGenConfig `json:"generationConfig"`
}

type geminiGenConfig struct {
	Temperature      float64 `json:"temperature"`
	MaxOutputTokens  int     `json:"maxOutputTokens"`
	ResponseMimeType string  `json:"responseMimeType,omitempty"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text    string `json:"text"`
				Thought bool   `json:"thought"`
			} `json:"parts"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
	PromptFeedback struct {
		BlockReason string `json:"blockReason"`
	} `json:"promptFeedback"`
}

func (g *gemini) Chat(ctx context.Context, req Request) (string, error) {
	body := geminiRequest{
		GenerationConfig: geminiGenConfig{Temperature: req.Temperature, MaxOutputTokens: req.MaxTokens},
	}
	if body.GenerationConfig.MaxOutputTokens <= 0 {
		body.GenerationConfig.MaxOutputTokens = 2048
	}
	if req.JSON {
		body.GenerationConfig.ResponseMimeType = "application/json"
	}

	var system []string
	for _, m := range req.Messages {
		switch m.Role {
		case RoleSystem:
			system = append(system, m.Content)
		case RoleAssistant:
			body.Contents = append(body.Contents, geminiContent{Role: "model", Parts: []geminiPart{{Text: m.Content}}})
		default:
			body.Contents = append(body.Contents, geminiContent{Role: "user", Parts: []geminiPart{{Text: m.Content}}})
		}
	}
	if len(system) > 0 {
		body.SystemInstruction = &geminiContent{Parts: []geminiPart{{Text: strings.Join(system, "\n\n")}}}
	}
	if len(body.Contents) == 0 {
		return "", fmt.Errorf("llm: request has no user or assistant messages")
	}

	endpoint := fmt.Sprintf("%s/v1beta/models/%s:generateContent", g.baseURL, url.PathEscape(g.model))
	var resp geminiResponse
	if err := g.http.postJSON(ctx, endpoint, map[string]string{"x-goog-api-key": g.apiKey}, body, &resp); err != nil {
		return "", err
	}

	if len(resp.Candidates) == 0 {
		if r := resp.PromptFeedback.BlockReason; r != "" {
			return "", fmt.Errorf("%w (blocked: %s)", ErrEmptyReply, r)
		}
		return "", ErrEmptyReply
	}

	var text strings.Builder
	for _, p := range resp.Candidates[0].Content.Parts {
		if !p.Thought { // skip the model's thinking notes
			text.WriteString(p.Text)
		}
	}
	out := strings.TrimSpace(text.String())
	if out == "" {
		if fr := resp.Candidates[0].FinishReason; fr != "" && fr != "STOP" {
			return "", fmt.Errorf("%w (finish reason: %s)", ErrEmptyReply, fr)
		}
		return "", ErrEmptyReply
	}
	return out, nil
}
