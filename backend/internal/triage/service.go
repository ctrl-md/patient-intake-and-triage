package triage

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/ctrl-md/patient-intake-and-triage/backend/internal/intake"
	"github.com/ctrl-md/patient-intake-and-triage/backend/internal/llm"
)

var ErrNotFound = errors.New("triage result not found")

// Store keeps triage results. Postgres in the app, a fake in tests.
type Store interface {
	// Save inserts or replaces the system part of the result (rules and AI).
	// It must not touch staff_urgency, override_reason or reviewed_*.
	Save(ctx context.Context, r Result) error
	Get(ctx context.Context, intakeID string) (Result, error)
}

type Service struct {
	store Store
	llm   llm.Client // nil means no AI. Rules still run.
}

func NewService(store Store, client llm.Client) *Service {
	return &Service{store: store, llm: client}
}

// PatientTexts picks out what the patient wrote.
func PatientTexts(msgs []intake.Message) []string {
	var out []string
	for _, m := range msgs {
		if m.Role == intake.RolePatient {
			out = append(out, m.Content)
		}
	}
	return out
}

func transcript(msgs []intake.Message) string {
	var b strings.Builder
	for _, m := range msgs {
		switch m.Role {
		case intake.RolePatient:
			b.WriteString("Patient: ")
		case intake.RoleAssistant:
			b.WriteString("Assistant: ")
		default:
			continue
		}
		b.WriteString(strings.TrimSpace(m.Content))
		b.WriteString("\n")
	}
	return b.String()
}

// Run triages one intake and saves the result. The rules always run first.
// A failing AI never stops the result from being saved.
func (s *Service) Run(ctx context.Context, intakeID string, msgs []intake.Message) (Result, error) {
	rulesLevel, flags := Evaluate(PatientTexts(msgs))

	res := Result{IntakeID: intakeID, RulesUrgency: rulesLevel, RedFlags: flags}

	if s.llm != nil {
		a, err := analyze(ctx, s.llm, transcript(msgs))
		if err != nil {
			log.Printf("triage: ai failed for %s: %v", intakeID, err) // patient text is not logged
		} else {
			res.AIUrgency = &a.Urgency
			res.AISummary = &a.Summary
			res.AIReasoning = &a.Reasoning
		}
	}
	res.FinalUrgency = Combine(rulesLevel, res.AIUrgency)

	if err := s.store.Save(ctx, res); err != nil {
		return Result{}, fmt.Errorf("save triage: %w", err)
	}
	return s.store.Get(ctx, intakeID)
}

func (s *Service) Get(ctx context.Context, intakeID string) (Result, error) {
	return s.store.Get(ctx, intakeID)
}

// IsEmergency is a quick check for the chat: do the patient's words hit an emergency rule?
func IsEmergency(patientTexts []string) bool {
	u, _ := Evaluate(patientTexts)
	return u == Emergency
}
