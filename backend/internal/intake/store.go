package intake

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound        = errors.New("intake not found")
	ErrTooManyMessages = errors.New("message limit reached")
)

const (
	StatusPending    = "pending"
	StatusInProgress = "in_progress"
	StatusCompleted  = "completed"
	StatusExpired    = "expired" // never stored. Worked out from expires_at.

	RolePatient   = "patient"
	RoleAssistant = "assistant"
	RoleSystem    = "system"
)

type Intake struct {
	ID           string
	PatientID    *string
	PatientName  *string
	Status       string // as stored: pending, in_progress, completed
	ExpiresAt    time.Time
	StartedAt    *time.Time
	CompletedAt  *time.Time
	CreatedAt    time.Time
	MessageCount int
	Urgency      *string // from triage: staff choice if any, else the system's. nil before triage.
}

type NewPatient struct {
	FullName    string
	Phone       *string
	DateOfBirth *time.Time
	Sex         *string
}

type NewIntake struct {
	TokenHash string
	CreatedBy string
	ExpiresAt time.Time
	Patient   *NewPatient // optional. Saved in the same step as the intake.
}

type Message struct {
	ID        string
	Role      string
	Content   string
	CreatedAt time.Time
}

// Store is where intakes live. Postgres in the app, a fake in tests.
type Store interface {
	CreateIntake(ctx context.Context, in NewIntake) (Intake, error)
	GetByTokenHash(ctx context.Context, hash string) (Intake, error)
	GetByID(ctx context.Context, id string) (Intake, error)
	// List returns newest first. An empty status means all.
	List(ctx context.Context, status string, limit int) ([]Intake, error)
	Messages(ctx context.Context, intakeID string) ([]Message, error)
	// AddMessage returns ErrTooManyMessages when the intake already has maxMessages.
	AddMessage(ctx context.Context, intakeID, role, content string, maxMessages int) (Message, error)
	// Start moves pending to in_progress. Does nothing for other statuses.
	Start(ctx context.Context, intakeID string) error
	// Complete moves pending or in_progress to completed.
	Complete(ctx context.Context, intakeID string) error
}

// effectiveStatus turns a stored status into what we show people.
// An unfinished intake whose link has run out is "expired".
func effectiveStatus(i Intake, now time.Time) string {
	unfinished := i.Status == StatusPending || i.Status == StatusInProgress
	if unfinished && !now.Before(i.ExpiresAt) {
		return StatusExpired
	}
	return i.Status
}
