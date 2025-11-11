package postgres

import (
	"context"
	"fmt"

	"github.com/Soyaib10/farm-fusion/internal/usecase/user"
	"github.com/Soyaib10/farm-fusion/internal/domain"
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

func (r *UserRepositoryPG) Create(user *domain.User) error {
	query := "INSERT INTO users (id, name, email, created_at) VALUES ($1, $2, $3, $4)"
	_, err := r.db.Exec(context.Background(), query, user.ID, user.Name, user.Email, user.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	return nil
}

func (r *UserRepositoryPG) GetByID(id string) (*domain.User, error) {
    query := "SELECT id, name, email, created_at FROM users WHERE id = $1"
    row := r.db.QueryRow(context.Background(), query, id)

    var user domain.User
    err := row.Scan(&user.ID, &user.Name, &user.Email, &user.CreatedAt)
    if err != nil {
        return nil, fmt.Errorf("failed to get user by id: %w", err)
    }

    return &user, nil
}