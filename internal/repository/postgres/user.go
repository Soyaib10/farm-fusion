package postgres

import (
	"context"
	"fmt"

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
		SELECT id, name, email, role, created_at
		FROM users
		WHERE id = $1
	`
	var (
		u    domain.User
		role string
	)
	if err := r.db.QueryRow(ctx, query, id).Scan(&u.ID, &u.Name, &u.Email, &role, &u.CreatedAt); err != nil {
		return nil, mapError("get user by id", err)
	}
	u.Role = domain.Role(role)
	return &u, nil
}

// GetByEmail fetches a user (including password hash) by case-insensitive email match.
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	const query = `
		SELECT id, name, email, role, password_hash, created_at
		FROM users
		WHERE LOWER(email) = LOWER($1)
	`
	var (
		u    domain.User
		role string
	)
	if err := r.db.QueryRow(ctx, query, email).Scan(
		&u.ID, &u.Name, &u.Email, &role, &u.PasswordHash, &u.CreatedAt,
	); err != nil {
		return nil, mapError("get user by email", err)
	}
	u.Role = domain.Role(role)
	return &u, nil
}

// Update modifies a user's mutable profile fields.
func (r *UserRepository) Update(ctx context.Context, u *domain.User) error {
	const query = `
		UPDATE users
		SET name = $1, email = $2
		WHERE id = $3
	`
	res, err := r.db.Exec(ctx, query, u.Name, u.Email, u.ID)
	if err != nil {
		return mapError("update user", err)
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("update user: %w", domain.ErrNotFound)
	}
	return nil
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
