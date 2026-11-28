package llm

import "context"

// Client is anything that can answer a chat request.
type Client interface {
	Chat(ctx context.Context, req Request) (string, error)
}
