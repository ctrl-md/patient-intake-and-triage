package intake

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PGStore struct {
	pool *pgxpool.Pool
}

func NewPGStore(pool *pgxpool.Pool) *PGStore {
	return &PGStore{pool: pool}
}

const intakeSelect = `
	SELECT i.id::text, i.patient_id::text, p.full_name, i.status,
	       i.expires_at, i.started_at, i.completed_at, i.created_at,
	       (SELECT count(*) FROM messages m WHERE m.intake_id = i.id)
	FROM intakes i
	LEFT JOIN patients p ON p.id = i.patient_id`

// CreateIntake saves the intake and, when given, the patient, in one statement.
func (s *PGStore) CreateIntake(ctx context.Context, in NewIntake) (Intake, error) {
	var name, phone, sex *string
	var dob *time.Time
	if in.Patient != nil {
		name, phone, dob, sex = &in.Patient.FullName, in.Patient.Phone, in.Patient.DateOfBirth, in.Patient.Sex
	}

	out := Intake{ExpiresAt: in.ExpiresAt, PatientName: name}
	err := s.pool.QueryRow(ctx, `
		WITH p AS (
			INSERT INTO patients (full_name, phone, date_of_birth, sex)
			SELECT $1::text, $2::text, $3::date, $4::text
			WHERE $1::text IS NOT NULL
			RETURNING id
		)
		INSERT INTO intakes (patient_id, created_by, token_hash, expires_at)
		VALUES ((SELECT id FROM p), $5::uuid, $6, $7)
		RETURNING id::text, patient_id::text, status, created_at`,
		name, phone, dob, sex, in.CreatedBy, in.TokenHash, in.ExpiresAt,
	).Scan(&out.ID, &out.PatientID, &out.Status, &out.CreatedAt)
	return out, err
}

func (s *PGStore) GetByTokenHash(ctx context.Context, hash string) (Intake, error) {
	return scanIntake(s.pool.QueryRow(ctx, intakeSelect+` WHERE i.token_hash = $1`, hash).Scan)
}

func (s *PGStore) GetByID(ctx context.Context, id string) (Intake, error) {
	return scanIntake(s.pool.QueryRow(ctx, intakeSelect+` WHERE i.id = $1::uuid`, id).Scan)
}

func (s *PGStore) List(ctx context.Context, status string, limit int) ([]Intake, error) {
	rows, err := s.pool.Query(ctx,
		intakeSelect+` WHERE ($1::text = '' OR i.status = $1::text) ORDER BY i.created_at DESC LIMIT $2`,
		status, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Intake
	for rows.Next() {
		it, err := scanIntake(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (s *PGStore) Messages(ctx context.Context, intakeID string) ([]Message, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, role, content, created_at
		FROM messages WHERE intake_id = $1::uuid
		ORDER BY created_at, id`, intakeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.Role, &m.Content, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// AddMessage inserts only if the intake has fewer than maxMessages messages.
// When the limit is reached, nothing is inserted and no row comes back.
func (s *PGStore) AddMessage(ctx context.Context, intakeID, role, content string, maxMessages int) (Message, error) {
	var m Message
	err := s.pool.QueryRow(ctx, `
		INSERT INTO messages (intake_id, role, content)
		SELECT $1::uuid, $2, $3
		WHERE (SELECT count(*) FROM messages WHERE intake_id = $1::uuid) < $4
		RETURNING id::text, role, content, created_at`,
		intakeID, role, content, maxMessages,
	).Scan(&m.ID, &m.Role, &m.Content, &m.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Message{}, ErrTooManyMessages
	}
	return m, err
}

func (s *PGStore) Start(ctx context.Context, intakeID string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE intakes SET status = 'in_progress', started_at = now()
		 WHERE id = $1::uuid AND status = 'pending'`, intakeID)
	return err
}

func (s *PGStore) Complete(ctx context.Context, intakeID string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE intakes SET status = 'completed', completed_at = now()
		 WHERE id = $1::uuid AND status IN ('pending', 'in_progress')`, intakeID)
	return err
}

func scanIntake(scan func(dest ...any) error) (Intake, error) {
	var it Intake
	err := scan(&it.ID, &it.PatientID, &it.PatientName, &it.Status,
		&it.ExpiresAt, &it.StartedAt, &it.CompletedAt, &it.CreatedAt, &it.MessageCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return Intake{}, ErrNotFound
	}
	return it, err
}
