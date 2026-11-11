package user

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/google/uuid"
)

type stubUserRepo struct {
	users map[uuid.UUID]*domain.User
}

func (s *stubUserRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.User, error) {
	u, ok := s.users[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *u
	return &cp, nil
}

func (s *stubUserRepo) Update(_ context.Context, u *domain.User) error {
	if _, ok := s.users[u.ID]; !ok {
		return domain.ErrNotFound
	}
	cp := *u
	s.users[u.ID] = &cp
	return nil
}

func (s *stubUserRepo) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := s.users[id]; !ok {
		return domain.ErrNotFound
	}
	delete(s.users, id)
	return nil
}

func (s *stubUserRepo) List(_ context.Context) ([]*domain.User, error) {
	out := make([]*domain.User, 0, len(s.users))
	for _, u := range s.users {
		cp := *u
		out = append(out, &cp)
	}
	return out, nil
}

func TestUserServiceAuthorizationPreventsBOLA(t *testing.T) {
	ctx := context.Background()
	aliceID := uuid.New()
	bobID := uuid.New()
	adminID := uuid.New()

	repo := &stubUserRepo{
		users: map[uuid.UUID]*domain.User{
			aliceID: {ID: aliceID, Name: "Alice", Email: "alice@example.com", Role: domain.RoleUser, CreatedAt: time.Now()},
			bobID:   {ID: bobID, Name: "Bob", Email: "bob@example.com", Role: domain.RoleUser, CreatedAt: time.Now()},
		},
	}
	svc := NewService(repo)

	aliceActor := domain.Actor{ID: aliceID, Role: domain.RoleUser}
	adminActor := domain.Actor{ID: adminID, Role: domain.RoleAdmin}

	// Alice cannot read, update, or delete Bob's account (BOLA/IDOR defense)
	if _, err := svc.GetByID(ctx, aliceActor, bobID); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("GetByID(bob) as alice got %v, want ErrForbidden", err)
	}
	hacked := "Hacked"
	if _, err := svc.Update(ctx, aliceActor, bobID, UpdateInput{Name: &hacked}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("Update(bob) as alice got %v, want ErrForbidden", err)
	}
	if err := svc.Delete(ctx, aliceActor, bobID); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("Delete(bob) as alice got %v, want ErrForbidden", err)
	}

	// Regular user cannot list all users
	if _, err := svc.List(ctx, aliceActor); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("List() as non-admin got %v, want ErrForbidden", err)
	}

	// Alice CAN read and update her own profile
	newName := "Alice Rahman"
	updated, err := svc.Update(ctx, aliceActor, aliceID, UpdateInput{Name: &newName})
	if err != nil {
		t.Fatalf("Update(self) unexpected error: %v", err)
	}
	if updated.Name != "Alice Rahman" {
		t.Fatalf("got updated name %q, want %q", updated.Name, "Alice Rahman")
	}

	// Admin CAN list all users and inspect Bob
	all, err := svc.List(ctx, adminActor)
	if err != nil || len(all) != 2 {
		t.Fatalf("List() as admin got len=%d err=%v, want 2 nil", len(all), err)
	}
}
