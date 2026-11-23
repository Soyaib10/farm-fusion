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
	pgCheckViolation      = "23514"
)

// checkConstraintFields maps CHECK constraint names to the request field they validate.
var checkConstraintFields = map[string]string{
	"valid_farm_coordinates":       "coordinates",
	"valid_weather_alert_operator": "operator",
	"valid_user_role":              "role",
}

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
			return fmt.Errorf("%s: %w", op, domain.ErrInvalidReference)
		case pgCheckViolation:
			field, ok := checkConstraintFields[pgErr.ConstraintName]
			if !ok {
				field = "body"
			}
			return fmt.Errorf("%s: %w", op, domain.NewValidationError(map[string]string{
				field: "violates a data constraint",
			}))
		}
	}
	return fmt.Errorf("%s: %w", op, err)
}
