package postgres

import (
	"context"
	"fmt"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/Soyaib10/farm-fusion/internal/usecase/user"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) user.Repository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, user *domain.User) error {
	query := `INSERT INTO users (id, name, email, password_hash, created_at) 
              VALUES ($1, $2, $3, $4, $5)`
	_, err := r.db.Exec(ctx, query, user.ID, user.Name, user.Email, user.PasswordHash, user.CreatedAt)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	query := `SELECT id, name, email, created_at FROM users WHERE id = $1`
	row := r.db.QueryRow(ctx, query, id)

	var u domain.User
	if err := row.Scan(&u.ID, &u.Name, &u.Email, &u.CreatedAt); err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return &u, nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	query := `SELECT id, name, email, password_hash, created_at FROM users WHERE email = $1`
	row := r.db.QueryRow(ctx, query, email)

	var u domain.User
	if err := row.Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.CreatedAt); err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return &u, nil
}

func (r *UserRepository) Update(ctx context.Context, user *domain.User) error {
	query := `UPDATE users SET name = $1, email = $2 WHERE id = $3`
	_, err := r.db.Exec(ctx, query, user.Name, user.Email, user.ID)
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	return nil
}

func (r *UserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM users WHERE id = $1`
	_, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	return nil
}

func (r *UserRepository) List(ctx context.Context) ([]*domain.User, error) {
	query := `SELECT id, name, email, created_at FROM users ORDER BY created_at DESC`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	var users []*domain.User
	for rows.Next() {
		var u domain.User
		if err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, &u)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return users, nil
}
