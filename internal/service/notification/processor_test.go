package notification

import (
	"context"
	"errors"
	"testing"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/google/uuid"
)

type countingSender struct {
	calls int
	err   error
}

func (s *countingSender) SendWeatherNotification(context.Context, *domain.NotificationPayload) (string, error) {
	s.calls++
	return "<p>alert</p>", s.err
}

type memDeliveries struct {
	sent       map[uuid.UUID]bool
	markSentOK bool
}

func newMemDeliveries() *memDeliveries {
	return &memDeliveries{sent: make(map[uuid.UUID]bool), markSentOK: true}
}

func (m *memDeliveries) Begin(_ context.Context, e *domain.NotificationLog) (bool, error) {
	sent, exists := m.sent[e.NotificationID]
	if !exists {
		m.sent[e.NotificationID] = false
	}
	return sent, nil
}

func (m *memDeliveries) MarkSent(_ context.Context, id uuid.UUID, _ string) error {
	if !m.markSentOK {
		return errors.New("database unavailable")
	}
	m.sent[id] = true
	return nil
}

func (m *memDeliveries) MarkFailed(context.Context, uuid.UUID, string, string) error { return nil }

type memRecipients map[uuid.UUID]*domain.User

func (m memRecipients) GetByID(_ context.Context, id uuid.UUID) (*domain.User, error) {
	u, ok := m[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return u, nil
}

func newPayload(userID uuid.UUID) *domain.NotificationPayload {
	return &domain.NotificationPayload{
		NotificationID: uuid.New(),
		FarmID:         uuid.New(),
		UserID:         userID,
		UserEmail:      "old@example.com",
		FarmName:       "North Field",
	}
}

func TestProcessorSameNotificationIDSendsOnce(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	sender := &countingSender{}
	p := NewProcessor(sender, newMemDeliveries(), memRecipients{userID: {ID: userID, Email: "farmer@example.com"}}, nil)

	payload := newPayload(userID)
	for i := 0; i < 3; i++ {
		redelivered := *payload
		if err := p.Process(ctx, &redelivered); err != nil {
			t.Fatalf("Process() attempt %d: %v", i+1, err)
		}
	}
	if sender.calls != 1 {
		t.Fatalf("redelivered notification sent %d times, want 1", sender.calls)
	}
}

func TestProcessorRetriesAfterFailedSendWithoutDuplicating(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	sender := &countingSender{err: errors.New("smtp down")}
	p := NewProcessor(sender, newMemDeliveries(), memRecipients{userID: {ID: userID, Email: "farmer@example.com"}}, nil)

	payload := newPayload(userID)
	if err := p.Process(ctx, payload); err == nil {
		t.Fatal("expected an error so the message is retried")
	}
	sender.err = nil
	if err := p.Process(ctx, payload); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if err := p.Process(ctx, payload); err != nil {
		t.Fatalf("redelivery after success: %v", err)
	}
	if sender.calls != 2 {
		t.Fatalf("sender called %d times, want 2 (one failure, one success)", sender.calls)
	}
}

func TestProcessorTreatsAcceptedMailAsSentWhenLogUpdateFails(t *testing.T) {
	userID := uuid.New()
	deliveries := newMemDeliveries()
	deliveries.markSentOK = false
	p := NewProcessor(&countingSender{}, deliveries, memRecipients{userID: {ID: userID, Email: "farmer@example.com"}}, nil)
	p.retryDelay = 0

	if err := p.Process(context.Background(), newPayload(userID)); err != nil {
		t.Fatalf("accepted mail must not be retried because of a log update failure, got %v", err)
	}
}

func TestProcessorSkipsDeletedRecipientAndUsesCurrentEmail(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	sender := &countingSender{}
	p := NewProcessor(sender, newMemDeliveries(), memRecipients{}, nil)

	if err := p.Process(ctx, newPayload(userID)); err != nil {
		t.Fatalf("Process() for deleted user: %v", err)
	}
	if sender.calls != 0 {
		t.Fatal("mail must not be sent to a deleted account")
	}

	p = NewProcessor(sender, newMemDeliveries(), memRecipients{userID: {ID: userID, Email: "new@example.com"}}, nil)
	payload := newPayload(userID)
	if err := p.Process(ctx, payload); err != nil {
		t.Fatal(err)
	}
	if payload.UserEmail != "new@example.com" {
		t.Fatalf("expected current account email, got %q", payload.UserEmail)
	}
}
