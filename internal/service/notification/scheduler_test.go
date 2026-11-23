package notification

import (
	"context"
	"testing"
	"time"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/google/uuid"
)

type fakeTargets struct{ targets []*domain.AlertTarget }

func (f fakeTargets) ListAlertTargets(_ context.Context, after domain.AlertTargetCursor, limit int) ([]*domain.AlertTarget, error) {
	var out []*domain.AlertTarget
	for _, t := range f.targets {
		k := domain.AlertTargetCursor{LocationKey: t.Farm.LocationKey, FarmID: t.Farm.ID}
		if after.LocationKey != "" && (k.LocationKey < after.LocationKey ||
			(k.LocationKey == after.LocationKey && k.FarmID.String() <= after.FarmID.String())) {
			continue
		}
		out = append(out, t)
		if len(out) == limit {
			break
		}
	}
	return out, nil
}

type fakeForecasts struct {
	calls    int
	forecast *domain.WeatherForecast
}

func (f *fakeForecasts) FetchForecast(_ context.Context, _, _ float64, key string) (*domain.WeatherForecast, error) {
	f.calls++
	cp := *f.forecast
	cp.LocationKey = key
	return &cp, nil
}

type fakePublisher struct {
	published []*domain.NotificationPayload
}

func (f *fakePublisher) Publish(_ context.Context, p *domain.NotificationPayload) error {
	f.published = append(f.published, p)
	return nil
}

type outboxEntry struct {
	id        uuid.UUID
	published bool
}

type fakeOutbox struct {
	entries map[string]*outboxEntry
}

func newFakeOutbox() *fakeOutbox { return &fakeOutbox{entries: make(map[string]*outboxEntry)} }

func (f *fakeOutbox) Reserve(_ context.Context, farmID uuid.UUID, day time.Time) (uuid.UUID, bool, error) {
	key := farmID.String() + day.Format("2006-01-02")
	e, ok := f.entries[key]
	if !ok {
		e = &outboxEntry{id: uuid.New()}
		f.entries[key] = e
	}
	return e.id, e.published, nil
}

func (f *fakeOutbox) MarkPublished(_ context.Context, id uuid.UUID) error {
	for _, e := range f.entries {
		if e.id == id {
			e.published = true
		}
	}
	return nil
}

func sampleTargets() (*domain.AlertTarget, *domain.AlertTarget) {
	userID := uuid.New()
	farm1 := domain.Farm{ID: uuid.New(), UserID: userID, Name: "Farm 1", Latitude: 23.8101, Longitude: 90.4121, LocationKey: "23.81_90.41"}
	farm2 := domain.Farm{ID: uuid.New(), UserID: userID, Name: "Farm 2", Latitude: 23.8140, Longitude: 90.4139, LocationKey: "23.81_90.41"}
	return &domain.AlertTarget{
			Farm: farm1, OwnerEmail: "farmer@example.com",
			Alerts: []*domain.Weather{{ID: uuid.New(), FarmID: farm1.ID, Metric: "temperature", Operator: ">", Value: 35, IsEnabled: true}},
		}, &domain.AlertTarget{
			Farm: farm2, OwnerEmail: "farmer@example.com",
			Alerts: []*domain.Weather{{ID: uuid.New(), FarmID: farm2.ID, Metric: "temperature", Operator: ">", Value: 40, IsEnabled: true}},
		}
}

func sampleForecast() *fakeForecasts {
	base := time.Date(2026, 11, 12, 6, 0, 0, 0, time.UTC)
	return &fakeForecasts{
		forecast: &domain.WeatherForecast{
			DataPoints: []domain.ForecastPoint{
				{Timestamp: base, Temperature: 37.0},
				{Timestamp: base.Add(3 * time.Hour), Temperature: 38.5},
				{Timestamp: base.Add(6 * time.Hour), Temperature: 28.0},
			},
		},
	}
}

func TestSchedulerDeduplicatesLocationKeyAndPublishesBreaches(t *testing.T) {
	ctx := context.Background()
	t1, t2 := sampleTargets()
	fp := sampleForecast()
	pub := &fakePublisher{}

	sched := NewScheduler(fakeTargets{targets: []*domain.AlertTarget{t1, t2}}, fp, pub, newFakeOutbox(), time.UTC)

	res, err := sched.RunScheduled(ctx)
	if err != nil {
		t.Fatalf("RunScheduled() unexpected error: %v", err)
	}
	if res.FarmsScanned != 2 || res.ForecastsFetched != 1 || fp.calls != 1 {
		t.Fatalf("expected 2 farms scanned and 1 shared forecast fetch, got %+v (calls=%d)", res, fp.calls)
	}
	if res.NotificationsPublished != 1 || len(pub.published) != 1 {
		t.Fatalf("expected 1 notification published, got %d", res.NotificationsPublished)
	}
	if pub.published[0].Alerts[0].Ranges[0].DurationHours != 6 {
		t.Fatalf("expected 6h contiguous breach window, got %d", pub.published[0].Alerts[0].Ranges[0].DurationHours)
	}
	if pub.published[0].NotificationID == uuid.Nil {
		t.Fatal("expected the outbox notification ID on the payload")
	}
}

func TestSchedulerSecondRunSameDayDoesNotRepublish(t *testing.T) {
	ctx := context.Background()
	t1, _ := sampleTargets()
	pub := &fakePublisher{}
	outbox := newFakeOutbox()
	sched := NewScheduler(fakeTargets{targets: []*domain.AlertTarget{t1}}, sampleForecast(), pub, outbox, time.UTC)

	if _, err := sched.RunScheduled(ctx); err != nil {
		t.Fatal(err)
	}
	res, err := sched.RunScheduled(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(pub.published) != 1 || res.AlreadyPublished != 1 {
		t.Fatalf("second run published again: published=%d result=%+v", len(pub.published), res)
	}
}

func TestCompareEqualityToleratesFloatNoise(t *testing.T) {
	if !compare(30.499999999, "=", 30.5) {
		t.Fatal("expected 30.499999999 = 30.5 to match")
	}
	if compare(30.4, "=", 30.5) {
		t.Fatal("expected 30.4 = 30.5 not to match")
	}
}
