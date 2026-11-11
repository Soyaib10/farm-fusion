package postgres

import (
	"errors"
	"fmt"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	pgUniqueViolation     = "23505"
	pgForeignKeyViolation = "23503"
)

func mapError(op string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%s: %w", op, domain.ErrNotFound)
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case pgUniqueViolation:
			return fmt.Errorf("%s: %w", op, domain.ErrAlreadyExists)
		case pgForeignKeyViolation:
			return fmt.Errorf("%s: %w", op, domain.ErrNotFound)
		}
	}
	return fmt.Errorf("%s: %w", op, err)
}
