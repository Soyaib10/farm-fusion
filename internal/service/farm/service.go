package farm

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/Soyaib10/farm-fusion/internal/validator"
	"github.com/google/uuid"
)

// Repository defines farm persistence operations needed by the farm service.
type Repository interface {
	Create(ctx context.Context, farm *domain.Farm) error
	GetByID(ctx context.Context, farmID uuid.UUID) (*domain.Farm, error)
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.Farm, error)
	Update(ctx context.Context, farm *domain.Farm) error
	Delete(ctx context.Context, farmID uuid.UUID) error
}

// UpsertInput contains validated parameters for creating or updating a farm.
type UpsertInput struct {
	Name      string
	Latitude  float64
	Longitude float64
}

// Service implements farm management and ownership enforcement.
type Service struct {
	repo Repository
}

// NewService constructs a farm Service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func validateUpsertInput(input UpsertInput) (string, error) {
	name := strings.TrimSpace(input.Name)

	v := validator.New()
	v.Check(validator.NotBlank(name), "name", "must be provided")
	v.Check(validator.RuneCountBetween(name, 3, 100), "name", "must be between 3 and 100 characters")
	v.Check(validator.NoControlChars(name), "name", "must not contain control characters")
	v.Check(validator.Between(input.Latitude, -90, 90), "latitude", "must be between -90 and 90")
	v.Check(validator.Between(input.Longitude, -180, 180), "longitude", "must be between -180 and 180")

	if err := v.Err(); err != nil {
		return "", err
	}
	return name, nil
}

// Create validates input and creates a farm owned by userID.
func (s *Service) Create(ctx context.Context, userID uuid.UUID, input UpsertInput) (*domain.Farm, error) {
	name, err := validateUpsertInput(input)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	f := &domain.Farm{
		ID:          uuid.New(),
		UserID:      userID,
		Name:        name,
		Latitude:    input.Latitude,
		Longitude:   input.Longitude,
		LocationKey: domain.NewLocationKey(input.Latitude, input.Longitude),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.repo.Create(ctx, f); err != nil {
		return nil, fmt.Errorf("create farm: %w", err)
	}
	return f, nil
}

// GetByID retrieves a farm and verifies ownership by userID.
func (s *Service) GetByID(ctx context.Context, farmID, userID uuid.UUID) (*domain.Farm, error) {
	f, err := s.repo.GetByID(ctx, farmID)
	if err != nil {
		return nil, err
	}
	if f.UserID != userID {
		return nil, domain.ErrForbidden
	}
	return f, nil
}

// ListByUserID returns all farms owned by userID.
func (s *Service) ListByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.Farm, error) {
	return s.repo.ListByUserID(ctx, userID)
}

// Update validates input, verifies farm ownership, and updates the farm record.
func (s *Service) Update(ctx context.Context, farmID, userID uuid.UUID, input UpsertInput) (*domain.Farm, error) {
	name, err := validateUpsertInput(input)
	if err != nil {
		return nil, err
	}

	existing, err := s.repo.GetByID(ctx, farmID)
	if err != nil {
		return nil, err
	}
	if existing.UserID != userID {
		return nil, domain.ErrForbidden
	}

	updated := &domain.Farm{
		ID:          farmID,
		UserID:      userID,
		Name:        name,
		Latitude:    input.Latitude,
		Longitude:   input.Longitude,
		LocationKey: domain.NewLocationKey(input.Latitude, input.Longitude),
		CreatedAt:   existing.CreatedAt,
		UpdatedAt:   time.Now().UTC(),
	}

	if err := s.repo.Update(ctx, updated); err != nil {
		return nil, fmt.Errorf("update farm: %w", err)
	}
	return updated, nil
}

// Delete verifies farm ownership and deletes the farm.
func (s *Service) Delete(ctx context.Context, farmID, userID uuid.UUID) error {
	existing, err := s.repo.GetByID(ctx, farmID)
	if err != nil {
		return err
	}
	if existing.UserID != userID {
		return domain.ErrForbidden
	}
	return s.repo.Delete(ctx, farmID)
}
