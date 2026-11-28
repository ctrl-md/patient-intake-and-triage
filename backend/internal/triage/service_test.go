package triage

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ctrl-md/patient-intake-and-triage/backend/internal/intake"
	"github.com/ctrl-md/patient-intake-and-triage/backend/internal/llm"
)

type memStore struct{ saved map[string]Result }

func (m *memStore) Save(_ context.Context, r Result) error {
	if m.saved == nil {
		m.saved = map[string]Result{}
	}
	m.saved[r.IntakeID] = r
	return nil
}
func (m *memStore) Get(_ context.Context, id string) (Result, error) {
	r, ok := m.saved[id]
	if !ok {
		return Result{}, ErrNotFound
	}
	return r, nil
}

func chatOf(patient ...string) []intake.Message {
	var out []intake.Message
	for _, p := range patient {
		out = append(out, intake.Message{Role: intake.RolePatient, Content: p},
			intake.Message{Role: intake.RoleAssistant, Content: "ok?"})
	}
	return out
}

const goodJSON = `{"summary":"Cough for two days.","urgency":"routine","reasoning":"Mild and short."}`

func TestParseAnalysisValid(t *testing.T) {
	for _, in := range []string{goodJSON, "```json\n" + goodJSON + "\n```", "  " + goodJSON + "\n"} {
		a, err := parseAnalysis(in)
		if err != nil || a.Urgency != Routine || a.Summary != "Cough for two days." {
			t.Fatalf("in=%q a=%+v err=%v", in, a, err)
		}
	}
}

func TestParseAnalysisRejects(t *testing.T) {
	bad := map[string]string{
		"not json":      `I think it is routine`,
		"bad urgency":   `{"summary":"a","urgency":"critical","reasoning":"b"}`,
		"empty urgency": `{"summary":"a","reasoning":"b"}`,
		"unknown key":   `{"summary":"a","urgency":"routine","reasoning":"b","diagnosis":"flu"}`,
		"missing":       `{"urgency":"routine","reasoning":"b"}`,
		"empty reason":  `{"summary":"a","urgency":"urgent","reasoning":"  "}`,
		"two objects":   goodJSON + goodJSON,
		"array":         `[` + goodJSON + `]`,
		"wrong type":    `{"summary":5,"urgency":"routine","reasoning":"b"}`,
		"too long":      `{"summary":"` + strings.Repeat("a", maxSummaryChars+1) + `","urgency":"routine","reasoning":"b"}`,
		"empty":         ``,
	}
	for name, in := range bad {
		if _, err := parseAnalysis(in); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestRunAIRaisesUrgency(t *testing.T) {
	st := &memStore{}
	f := llm.NewFake(llm.Say(`{"summary":"Bad belly pain.","urgency":"urgent","reasoning":"Needs a look today."}`))
	res, err := NewService(st, f).Run(context.Background(), "i1", chatOf("my stomach hurts"))
	if err != nil {
		t.Fatal(err)
	}
	if res.RulesUrgency != Routine || res.FinalUrgency != Urgent || res.AIUrgency == nil || *res.AIUrgency != Urgent {
		t.Fatalf("%+v", res)
	}
}

func TestRunAICannotLowerRules(t *testing.T) {
	st := &memStore{}
	f := llm.NewFake(llm.Say(goodJSON)) // AI says routine
	res, _ := NewService(st, f).Run(context.Background(), "i1", chatOf("I have chest pain"))
	if res.FinalUrgency != Emergency || res.RulesUrgency != Emergency || len(res.RedFlags) == 0 {
		t.Fatalf("%+v", res)
	}
}

func TestRunAIFailureBumpsUp(t *testing.T) {
	st := &memStore{}
	f := llm.NewFake(llm.Fail(errors.New("provider down")))
	res, err := NewService(st, f).Run(context.Background(), "i1", chatOf("a cough"))
	if err != nil {
		t.Fatal(err)
	}
	if res.AIUrgency != nil || res.AISummary != nil || res.FinalUrgency != Urgent {
		t.Fatalf("%+v", res)
	}
}

func TestRunRetriesOnceOnBadJSON(t *testing.T) {
	st := &memStore{}
	f := llm.NewFake(llm.Say("sure! it is fine"), llm.Say(goodJSON))
	res, _ := NewService(st, f).Run(context.Background(), "i1", chatOf("a cough"))
	if res.AIUrgency == nil || len(f.Calls()) != 2 {
		t.Fatalf("res=%+v calls=%d", res, len(f.Calls()))
	}
	last := f.Calls()[1].Messages
	if !strings.Contains(last[len(last)-1].Content, "rejected") {
		t.Fatal("second call should say why the first was rejected")
	}
}

func TestRunGivesUpAfterTwoBadReplies(t *testing.T) {
	st := &memStore{}
	f := llm.NewFake(llm.Say("nope"), llm.Say("still nope"))
	res, _ := NewService(st, f).Run(context.Background(), "i1", chatOf("a cough"))
	if res.AIUrgency != nil || res.FinalUrgency != Urgent || len(f.Calls()) != 2 {
		t.Fatalf("%+v calls=%d", res, len(f.Calls()))
	}
}

func TestRunWithoutAIClient(t *testing.T) {
	st := &memStore{}
	res, _ := NewService(st, nil).Run(context.Background(), "i1", chatOf("a cough"))
	if res.FinalUrgency != Urgent || res.AIUrgency != nil {
		t.Fatalf("%+v", res)
	}
}

func TestPromptHasOnlyPatientChatNotRules(t *testing.T) {
	f := llm.NewFake(llm.Say(goodJSON))
	NewService(&memStore{}, f).Run(context.Background(), "i1", append(chatOf("a cough"),
		intake.Message{Role: intake.RoleSystem, Content: "staff note: secret"}))
	user := f.Calls()[0].Messages[1].Content
	if strings.Contains(user, "secret") || !strings.Contains(user, "Patient: a cough") {
		t.Fatalf("transcript wrong: %q", user)
	}
	if !f.Calls()[0].JSON {
		t.Fatal("JSON mode should be on")
	}
}

func TestOnlyPatientWordsFeedRules(t *testing.T) {
	// The assistant mentioning "chest pain" must not trigger the rules.
	msgs := []intake.Message{
		{Role: intake.RolePatient, Content: "I feel tired"},
		{Role: intake.RoleAssistant, Content: "Do you have chest pain?"},
	}
	res, _ := NewService(&memStore{}, llm.NewFake(llm.Say(goodJSON))).Run(context.Background(), "i1", msgs)
	if res.RulesUrgency != Routine {
		t.Fatalf("assistant text leaked into rules: %+v", res)
	}
}
