package auth

import (
	"context"
	"errors"
	"time"
)

var (
	ErrEmailTaken = errors.New("email already registered")
	ErrNotFound   = errors.New("staff not found")
)

type Staff struct {
	ID           string
	Email        string
	PasswordHash string
	FullName     string
	Role         string
	CreatedAt    time.Time
}

type NewStaff struct {
	Email        string
	PasswordHash string
	FullName     string
}

// StaffStore is where staff accounts live. Postgres in the app, a fake in tests.
type StaffStore interface {
	Create(ctx context.Context, s NewStaff) (Staff, error)
	GetByEmail(ctx context.Context, email string) (Staff, error)
	GetByID(ctx context.Context, id string) (Staff, error)
}
