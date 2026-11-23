package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// UserRepository persists and queries domain.User records in PostgreSQL.
type UserRepository struct {
	db *pgxpool.Pool
}

// NewUserRepository constructs a PostgreSQL UserRepository.
func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

// Create inserts a new user row.
func (r *UserRepository) Create(ctx context.Context, u *domain.User) error {
	role := u.Role
	if role == "" {
		role = domain.RoleUser
	}
	const query = `
		INSERT INTO users (id, name, email, role, password_hash, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.db.Exec(ctx, query, u.ID, u.Name, u.Email, string(role), u.PasswordHash, u.CreatedAt)
	return mapError("create user", err)
}

// GetByID fetches a user by primary key.
func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	const query = `
		SELECT id, name, email, role, created_at, token_version
		FROM users
		WHERE id = $1
	`
	var (
		u    domain.User
		role string
	)
	if err := r.db.QueryRow(ctx, query, id).Scan(&u.ID, &u.Name, &u.Email, &role, &u.CreatedAt, &u.TokenVersion); err != nil {
		return nil, mapError("get user by id", err)
	}
	u.Role = domain.Role(role)
	return &u, nil
}

// GetByEmail fetches a user (including password hash and lockout state) by case-insensitive email match.
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	const query = `
		SELECT id, name, email, role, password_hash, created_at, token_version, failed_login_count, locked_until
		FROM users
		WHERE LOWER(email) = LOWER($1)
	`
	var (
		u    domain.User
		role string
	)
	if err := r.db.QueryRow(ctx, query, email).Scan(
		&u.ID, &u.Name, &u.Email, &role, &u.PasswordHash, &u.CreatedAt,
		&u.TokenVersion, &u.FailedLoginCount, &u.LockedUntil,
	); err != nil {
		return nil, mapError("get user by email", err)
	}
	u.Role = domain.Role(role)
	return &u, nil
}

// GetPasswordHash returns the bcrypt hash stored for id.
func (r *UserRepository) GetPasswordHash(ctx context.Context, id uuid.UUID) (string, error) {
	var hash string
	if err := r.db.QueryRow(ctx, `SELECT password_hash FROM users WHERE id = $1`, id).Scan(&hash); err != nil {
		return "", mapError("get password hash", err)
	}
	return hash, nil
}

// RecordLoginFailure increments the failed-login counter and locks the account for lockout once
// the counter reaches maxFailures. The counter is shared by every API instance.
func (r *UserRepository) RecordLoginFailure(ctx context.Context, id uuid.UUID, maxFailures int, lockout time.Duration) error {
	const query = `
		UPDATE users
		SET failed_login_count = failed_login_count + 1,
		    locked_until = CASE
		        WHEN failed_login_count + 1 >= $2 THEN NOW() + make_interval(secs => $3)
		        ELSE locked_until
		    END
		WHERE id = $1
	`
	_, err := r.db.Exec(ctx, query, id, maxFailures, lockout.Seconds())
	return mapError("record login failure", err)
}

// ResetLoginFailures clears the failed-login counter and any lock after a successful login.
func (r *UserRepository) ResetLoginFailures(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET failed_login_count = 0, locked_until = NULL WHERE id = $1`, id)
	return mapError("reset login failures", err)
}

// UpdateProfile writes only the provided fields, so concurrent updates of different fields do not
// overwrite each other, and returns the resulting row.
func (r *UserRepository) UpdateProfile(ctx context.Context, id uuid.UUID, name, email *string) (*domain.User, error) {
	const query = `
		UPDATE users
		SET name = COALESCE($1, name), email = COALESCE($2, email)
		WHERE id = $3
		RETURNING id, name, email, role, created_at, token_version
	`
	var (
		u    domain.User
		role string
	)
	if err := r.db.QueryRow(ctx, query, name, email, id).Scan(
		&u.ID, &u.Name, &u.Email, &role, &u.CreatedAt, &u.TokenVersion,
	); err != nil {
		return nil, mapError("update user", err)
	}
	u.Role = domain.Role(role)
	return &u, nil
}

// Delete removes a user by ID.
func (r *UserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	const query = `DELETE FROM users WHERE id = $1`
	res, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return mapError("delete user", err)
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("delete user: %w", domain.ErrNotFound)
	}
	return nil
}

// List returns all users ordered newest first.
func (r *UserRepository) List(ctx context.Context) ([]*domain.User, error) {
	const query = `
		SELECT id, name, email, role, created_at
		FROM users
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, mapError("list users", err)
	}
	defer rows.Close()

	var users []*domain.User
	for rows.Next() {
		var (
			u    domain.User
			role string
		)
		if err := rows.Scan(&u.ID, &u.Name, &u.Email, &role, &u.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		u.Role = domain.Role(role)
		users = append(users, &u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate users: %w", err)
	}
	return users, nil
}
