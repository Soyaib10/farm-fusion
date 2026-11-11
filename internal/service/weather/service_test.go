package weather

import (
	"context"
	"errors"
	"testing"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/google/uuid"
)

type stubWeatherRepo struct {
	rules map[uuid.UUID]*domain.Weather
}

func (s *stubWeatherRepo) Create(_ context.Context, w *domain.Weather) error {
	cp := *w
	s.rules[w.ID] = &cp
	return nil
}
func (s *stubWeatherRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.Weather, error) {
	w, ok := s.rules[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *w
	return &cp, nil
}
func (s *stubWeatherRepo) ListByFarmID(_ context.Context, farmID uuid.UUID) ([]*domain.Weather, error) {
	var out []*domain.Weather
	for _, w := range s.rules {
		if w.FarmID == farmID {
			cp := *w
			out = append(out, &cp)
		}
	}
	return out, nil
}
func (s *stubWeatherRepo) Update(_ context.Context, w *domain.Weather) error {
	cp := *w
	s.rules[w.ID] = &cp
	return nil
}
func (s *stubWeatherRepo) Delete(_ context.Context, id uuid.UUID) error {
	delete(s.rules, id)
	return nil
}

type stubFarmChecker struct {
	ownerByFarm map[uuid.UUID]uuid.UUID
}

func (s *stubFarmChecker) GetByID(_ context.Context, farmID, userID uuid.UUID) (*domain.Farm, error) {
	owner, ok := s.ownerByFarm[farmID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	if owner != userID {
		return nil, domain.ErrForbidden
	}
	return &domain.Farm{ID: farmID, UserID: owner}, nil
}

func TestWeatherServiceValidationAndFarmAssociation(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	farmA := uuid.New()
	farmB := uuid.New()

	repo := &stubWeatherRepo{rules: make(map[uuid.UUID]*domain.Weather)}
	farms := &stubFarmChecker{ownerByFarm: map[uuid.UUID]uuid.UUID{
		farmA: userID,
		farmB: userID,
	}}
	svc := NewService(repo, farms)

	// Invalid metric or operator rejected
	if _, err := svc.Create(ctx, CreateInput{
		UserID: userID, FarmID: farmA, Metric: "solar_flare", Operator: ">", Value: 10,
	}); err == nil {
		t.Fatal("expected validation error for unsupported metric")
	}
	if _, err := svc.Create(ctx, CreateInput{
		UserID: userID, FarmID: farmA, Metric: "humidity", Operator: ">", Value: 150,
	}); err == nil {
		t.Fatal("expected validation error for humidity > 100")
	}

	rule, err := svc.Create(ctx, CreateInput{
		UserID: userID, FarmID: farmA, Metric: "temp", Operator: "<", Value: 0,
	})
	if err != nil {
		t.Fatalf("Create() unexpected error: %v", err)
	}
	if rule.Metric != "temperature" {
		t.Fatalf("expected metric normalization temp -> temperature, got %q", rule.Metric)
	}

	// Accessing Farm A's rule via Farm B's URL returns ErrNotFound
	if _, err := svc.GetByID(ctx, userID, farmB, rule.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("cross-farm GetByID got %v, want ErrNotFound", err)
	}
}
