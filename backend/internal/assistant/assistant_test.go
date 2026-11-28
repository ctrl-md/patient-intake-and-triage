package assistant

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ctrl-md/patient-intake-and-triage/backend/internal/intake"
	"github.com/ctrl-md/patient-intake-and-triage/backend/internal/llm"
)

func chat(roles ...string) []intake.Message {
	var out []intake.Message
	for i, r := range roles {
		out = append(out, intake.Message{Role: r, Content: "m" + string(rune('a'+i))})
	}
	return out
}

func TestReplyUsesAI(t *testing.T) {
	f := llm.NewFake(llm.Say("  How long has it lasted?  "))
	got := New(f).Reply(context.Background(), chat(intake.RolePatient, intake.RoleAssistant, intake.RolePatient))
	if got != "How long has it lasted?" {
		t.Fatalf("got %q", got)
	}
	req := f.Calls()[0]
	if req.Messages[0].Role != llm.RoleSystem || len(req.Messages) != 4 {
		t.Fatalf("bad request: %+v", req.Messages)
	}
	if req.Messages[1].Role != llm.RoleUser || req.Messages[2].Role != llm.RoleAssistant {
		t.Fatal("roles not mapped")
	}
}

func TestFallbackWhenAIFails(t *testing.T) {
	a := New(llm.NewFake(llm.Fail(errors.New("down"))))
	got := a.Reply(context.Background(), chat(intake.RolePatient))
	if got != fallbackQuestions[0] {
		t.Fatalf("got %q", got)
	}
}

func TestFallbackWithoutAI(t *testing.T) {
	a := New(nil)
	h := chat(intake.RolePatient, intake.RoleAssistant, intake.RolePatient)
	if got := a.Reply(context.Background(), h); got != fallbackQuestions[1] {
		t.Fatalf("got %q", got)
	}
	many := make([]string, 20)
	for i := range many {
		many[i] = intake.RolePatient
	}
	if got := a.Reply(context.Background(), chat(many...)); got != fallbackQuestions[len(fallbackQuestions)-1] {
		t.Fatalf("got %q", got)
	}
}

func TestEmptyAIReplyFallsBack(t *testing.T) {
	a := New(llm.NewFake(llm.Say("   ")))
	if got := a.Reply(context.Background(), chat(intake.RolePatient)); got != fallbackQuestions[0] {
		t.Fatalf("got %q", got)
	}
}

func TestLongReplyIsCut(t *testing.T) {
	a := New(llm.NewFake(llm.Say(strings.Repeat("é", 2000))))
	got := a.Reply(context.Background(), chat(intake.RolePatient))
	if len([]rune(got)) > maxReplyChars+3 {
		t.Fatalf("too long: %d", len([]rune(got)))
	}
}
