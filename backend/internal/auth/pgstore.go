package auth

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// id is cast to text so it scans straight into a Go string.
const staffColumns = `id::text, email, password_hash, full_name, role, created_at`

type PGStore struct {
	pool *pgxpool.Pool
}

func NewPGStore(pool *pgxpool.Pool) *PGStore {
	return &PGStore{pool: pool}
}

func (s *PGStore) Create(ctx context.Context, in NewStaff) (Staff, error) {
	row := s.pool.QueryRow(ctx,
		`INSERT INTO staff (email, password_hash, full_name)
		 VALUES ($1, $2, $3)
		 RETURNING `+staffColumns,
		in.Email, in.PasswordHash, in.FullName)

	st, err := scanStaff(row)
	if err != nil {
		var pgErr *pgconn.PgError
		// 23505 = unique violation (the lower(email) index)
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Staff{}, ErrEmailTaken
		}
		return Staff{}, err
	}
	return st, nil
}

func (s *PGStore) GetByEmail(ctx context.Context, email string) (Staff, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT `+staffColumns+` FROM staff WHERE lower(email) = lower($1)`, email)
	return scanStaff(row)
}

func (s *PGStore) GetByID(ctx context.Context, id string) (Staff, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT `+staffColumns+` FROM staff WHERE id = $1::uuid`, id)
	return scanStaff(row)
}

func scanStaff(row pgx.Row) (Staff, error) {
	var st Staff
	err := row.Scan(&st.ID, &st.Email, &st.PasswordHash, &st.FullName, &st.Role, &st.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Staff{}, ErrNotFound
	}
	return st, err
}
