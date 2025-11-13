package postgres

import (
	"context"
	"fmt"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/Soyaib10/farm-fusion/internal/usecase/user"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepositoryPG struct {
	db *pgxpool.Pool
}

func NewUserRepositoryPG(db *pgxpool.Pool) user.Repository {
	return &UserRepositoryPG{
		db: db,
	}
}

func (r *UserRepositoryPG) Create(ctx context.Context, user *domain.User) error {
	query := "INSERT INTO users (id, name, email, created_at) VALUES ($1, $2, $3, $4)"
	_, err := r.db.Exec(context.Background(), query, user.ID, user.Name, user.Email, user.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	return nil
}

func (r *UserRepositoryPG) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	query := "SELECT id, name, email, created_at FROM users WHERE id = $1"
	row := r.db.QueryRow(context.Background(), query, id)

	var user domain.User
	err := row.Scan(&user.ID, &user.Name, &user.Email, &user.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to get user by id: %w", err)
	}

	return &user, nil
}

func (r *UserRepositoryPG) Update(ctx context.Context, user *domain.User) error {
	query := "UPDATE users SET name = $1, email = $2 WHERE id = $3"
	_, err := r.db.Exec(context.Background(), query, user.Name, user.Email, user.ID)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}
	return nil
}

func (r *UserRepositoryPG) Delete(ctx context.Context, id uuid.UUID) error {
	query := "DELETE FROM users WHERE id = $1"
	_, err := r.db.Exec(context.Background(), query, id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}
	return nil
}

func (r *UserRepositoryPG) Find(ctx context.Context) ([]domain.User, error) {
	query := "SELECT id, name, email, created_at FROM users"
	rows, err := r.db.Query(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("failed to find users: %w", err)
	}
	defer rows.Close()

	var users []domain.User
	for rows.Next() {
		var user domain.User
		err := rows.Scan(&user.ID, &user.Name, &user.Email, &user.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, user)
	}

	return users, nil
}