package user

import (
	"context"
	"errors"
	"strings"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/Soyaib10/farm-fusion/internal/validator"
	"github.com/google/uuid"
)

// Repository defines user persistence operations needed by the user service.
type Repository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	GetPasswordHash(ctx context.Context, id uuid.UUID) (string, error)
	UpdateProfile(ctx context.Context, id uuid.UUID, name, email *string) (*domain.User, error)
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context) ([]*domain.User, error)
}

// UpdateInput contains optional profile fields to update.
type UpdateInput struct {
	Name  *string
	Email *string
	// CurrentPassword is the acting user's password. It is required to change an email address,
	// so a stolen access token alone cannot take over the account's login identity.
	CurrentPassword *string
}

// Service implements authorization-checked user profile management.
type Service struct {
	repo Repository
}

// NewService constructs a user Service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// GetByID retrieves a user profile if actor owns the account or is an admin.
func (s *Service) GetByID(ctx context.Context, actor domain.Actor, targetID uuid.UUID) (*domain.User, error) {
	if !actor.CanAccessUser(targetID) {
		return nil, domain.ErrForbidden
	}
	return s.repo.GetByID(ctx, targetID)
}

// Update modifies a user profile after verifying ownership/admin access and validating fields.
func (s *Service) Update(ctx context.Context, actor domain.Actor, targetID uuid.UUID, input UpdateInput) (*domain.User, error) {
	if !actor.CanAccessUser(targetID) {
		return nil, domain.ErrForbidden
	}

	v := validator.New()
	if input.Name == nil && input.Email == nil {
		v.AddError("body", "at least one of name or email must be provided")
		return nil, v.Err()
	}

	var name, email *string
	if input.Name != nil {
		cleanName := strings.TrimSpace(*input.Name)
		v.Check(validator.NotBlank(cleanName), "name", "must not be empty")
		v.Check(validator.RuneCountBetween(cleanName, 2, 100), "name", "must be between 2 and 100 characters")
		v.Check(validator.NoControlChars(cleanName), "name", "must not contain control characters")
		name = &cleanName
	}
	if input.Email != nil {
		cleanEmail := strings.ToLower(strings.TrimSpace(*input.Email))
		v.Check(validator.NotBlank(cleanEmail), "email", "must not be empty")
		v.Check(len(cleanEmail) <= 254, "email", "must not exceed 254 characters")
		v.Check(validator.NoControlChars(cleanEmail), "email", "must not contain control characters")
		v.Check(validator.Matches(cleanEmail, validator.EmailRX), "email", "must be a valid email address")
		v.Check(input.CurrentPassword != nil && *input.CurrentPassword != "", "current_password",
			"must be provided to change the email address")
		email = &cleanEmail
	}
	if err := v.Err(); err != nil {
		return nil, err
	}

	if email != nil {
		if err := s.verifyActorPassword(ctx, actor, *input.CurrentPassword); err != nil {
			return nil, err
		}
	}

	return s.repo.UpdateProfile(ctx, targetID, name, email)
}

func (s *Service) verifyActorPassword(ctx context.Context, actor domain.Actor, password string) error {
	hash, err := s.repo.GetPasswordHash(ctx, actor.ID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.ErrUnauthorized
		}
		return err
	}
	u := domain.User{PasswordHash: hash}
	if !u.CheckPassword(password) {
		return domain.NewValidationError(map[string]string{"current_password": "is incorrect"})
	}
	return nil
}

// Delete deletes a user account if actor owns the account or is an admin.
func (s *Service) Delete(ctx context.Context, actor domain.Actor, targetID uuid.UUID) error {
	if !actor.CanAccessUser(targetID) {
		return domain.ErrForbidden
	}
	return s.repo.Delete(ctx, targetID)
}

// List returns all users; restricted strictly to administrators to prevent PII enumeration.
func (s *Service) List(ctx context.Context, actor domain.Actor) ([]*domain.User, error) {
	if !actor.IsAdmin() {
		return nil, domain.ErrForbidden
	}
	return s.repo.List(ctx)
}
