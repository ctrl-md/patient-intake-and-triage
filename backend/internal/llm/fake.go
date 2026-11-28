package llm

import (
	"context"
	"errors"
	"sync"
)

// FakeReply is one scripted answer for Fake.
type FakeReply struct {
	Text string
	Err  error
}

// Fake is a Client for tests. It returns scripted replies in order and
// remembers every request it got.
type Fake struct {
	mu      sync.Mutex
	replies []FakeReply
	calls   []Request
}

func NewFake(replies ...FakeReply) *Fake { return &Fake{replies: replies} }

// Say is a shortcut for a text reply.
func Say(text string) FakeReply { return FakeReply{Text: text} }

// Fail is a shortcut for an error reply.
func Fail(err error) FakeReply { return FakeReply{Err: err} }

func (f *Fake) Chat(_ context.Context, req Request) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, req)
	if len(f.replies) == 0 {
		return "", errors.New("llm.Fake: no scripted reply left")
	}
	r := f.replies[0]
	f.replies = f.replies[1:]
	return r.Text, r.Err
}

// Calls returns a copy of all requests so far.
func (f *Fake) Calls() []Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Request(nil), f.calls...)
}
