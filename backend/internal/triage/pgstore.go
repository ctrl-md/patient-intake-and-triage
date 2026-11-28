package triage

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PGStore struct{ pool *pgxpool.Pool }

func NewPGStore(pool *pgxpool.Pool) *PGStore { return &PGStore{pool: pool} }

// Save writes the rules and AI part. On a re-run it replaces them and leaves
// the staff override alone. final_urgency is the system's answer, so staff
// changes live in staff_urgency.
func (s *PGStore) Save(ctx context.Context, r Result) error {
	flags, err := json.Marshal(r.RedFlags)
	if err != nil {
		return err
	}
	var aiUrgency *string
	if r.AIUrgency != nil {
		v := string(*r.AIUrgency)
		aiUrgency = &v
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO triage_results
			(intake_id, rules_urgency, red_flags, ai_urgency, ai_summary, ai_reasoning, final_urgency)
		VALUES ($1::uuid, $2, $3::jsonb, $4, $5, $6, $7)
		ON CONFLICT (intake_id) DO UPDATE SET
			rules_urgency = EXCLUDED.rules_urgency,
			red_flags     = EXCLUDED.red_flags,
			ai_urgency    = EXCLUDED.ai_urgency,
			ai_summary    = EXCLUDED.ai_summary,
			ai_reasoning  = EXCLUDED.ai_reasoning,
			final_urgency = EXCLUDED.final_urgency`,
		r.IntakeID, string(r.RulesUrgency), string(flags), aiUrgency, r.AISummary, r.AIReasoning, string(r.FinalUrgency))
	return err
}

func (s *PGStore) Get(ctx context.Context, intakeID string) (Result, error) {
	var r Result
	var rules, final string
	var ai, staff *string
	var flags []byte
	err := s.pool.QueryRow(ctx, `
		SELECT intake_id::text, rules_urgency, red_flags, ai_urgency, ai_summary, ai_reasoning,
		       final_urgency, staff_urgency, override_reason, reviewed_at, created_at
		FROM triage_results WHERE intake_id = $1::uuid`, intakeID,
	).Scan(&r.IntakeID, &rules, &flags, &ai, &r.AISummary, &r.AIReasoning,
		&final, &staff, &r.Override, &r.ReviewedAt, &r.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Result{}, ErrNotFound
	}
	if err != nil {
		return Result{}, err
	}
	r.RulesUrgency, r.FinalUrgency = Urgency(rules), Urgency(final)
	if ai != nil {
		u := Urgency(*ai)
		r.AIUrgency = &u
	}
	if staff != nil {
		u := Urgency(*staff)
		r.StaffUrgency = &u
	}
	r.RedFlags = []RedFlag{}
	if err := json.Unmarshal(flags, &r.RedFlags); err != nil {
		return Result{}, err
	}
	return r, nil
}
