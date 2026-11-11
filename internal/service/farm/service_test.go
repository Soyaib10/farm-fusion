package farm

import (
	"context"
	"errors"
	"testing"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/google/uuid"
)

type memFarmRepo struct {
	farms map[uuid.UUID]*domain.Farm
}

func (m *memFarmRepo) Create(_ context.Context, f *domain.Farm) error {
	cp := *f
	m.farms[f.ID] = &cp
	return nil
}

func (m *memFarmRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.Farm, error) {
	f, ok := m.farms[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *f
	return &cp, nil
}

func (m *memFarmRepo) ListByUserID(_ context.Context, userID uuid.UUID) ([]*domain.Farm, error) {
	var out []*domain.Farm
	for _, f := range m.farms {
		if f.UserID == userID {
			cp := *f
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (m *memFarmRepo) Update(_ context.Context, f *domain.Farm) error {
	cp := *f
	m.farms[f.ID] = &cp
	return nil
}

func (m *memFarmRepo) Delete(_ context.Context, id uuid.UUID) error {
	delete(m.farms, id)
	return nil
}

func TestFarmServiceValidationAndOwnership(t *testing.T) {
	ctx := context.Background()
	repo := &memFarmRepo{farms: make(map[uuid.UUID]*domain.Farm)}
	svc := NewService(repo)

	ownerID := uuid.New()
	intruderID := uuid.New()

	// Rejects out-of-range coordinates & CRLF names
	if _, err := svc.Create(ctx, ownerID, UpsertInput{Name: "BadLat", Latitude: 95, Longitude: 90}); err == nil {
		t.Fatal("expected validation error for latitude > 90")
	}
	if _, err := svc.Create(ctx, ownerID, UpsertInput{Name: "Inject\r\nHeader", Latitude: 23.81, Longitude: 90.41}); err == nil {
		t.Fatal("expected validation error for control characters in farm name")
	}

	created, err := svc.Create(ctx, ownerID, UpsertInput{
		Name:      "Padma Orchard",
		Latitude:  23.8103,
		Longitude: 90.4125,
	})
	if err != nil {
		t.Fatalf("Create() unexpected error: %v", err)
	}
	if created.LocationKey != "23.81_90.41" {
		t.Fatalf("got LocationKey %q, want 23.81_90.41", created.LocationKey)
	}

	// Intruder cannot read, update, or delete owner's farm
	if _, err := svc.GetByID(ctx, created.ID, intruderID); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("GetByID as intruder got %v, want ErrForbidden", err)
	}
	if err := svc.Delete(ctx, created.ID, intruderID); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("Delete as intruder got %v, want ErrForbidden", err)
	}
}
