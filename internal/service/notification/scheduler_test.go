package notification

import (
	"context"
	"testing"
	"time"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/google/uuid"
)

type fakeFarms struct{ farms []*domain.Farm }

func (f fakeFarms) ListAll(context.Context) ([]*domain.Farm, error) { return f.farms, nil }

type fakeAlerts struct {
	byFarm map[uuid.UUID][]*domain.Weather
}

func (f fakeAlerts) ListByFarmID(_ context.Context, id uuid.UUID) ([]*domain.Weather, error) {
	return f.byFarm[id], nil
}

type fakeUsers struct {
	byID map[uuid.UUID]*domain.User
}

func (f fakeUsers) GetByID(_ context.Context, id uuid.UUID) (*domain.User, error) {
	return f.byID[id], nil
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

func TestSchedulerDeduplicatesLocationKeyAndPublishesBreaches(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	farm1 := &domain.Farm{ID: uuid.New(), UserID: userID, Name: "Farm 1", Latitude: 23.8101, Longitude: 90.4121, LocationKey: "23.81_90.41"}
	farm2 := &domain.Farm{ID: uuid.New(), UserID: userID, Name: "Farm 2", Latitude: 23.8140, Longitude: 90.4139, LocationKey: "23.81_90.41"}

	base := time.Date(2026, 11, 12, 6, 0, 0, 0, time.UTC)
	fp := &fakeForecasts{
		forecast: &domain.WeatherForecast{
			DataPoints: []domain.ForecastPoint{
				{Timestamp: base, Temperature: 37.0},
				{Timestamp: base.Add(3 * time.Hour), Temperature: 38.5},
				{Timestamp: base.Add(6 * time.Hour), Temperature: 28.0},
			},
		},
	}
	pub := &fakePublisher{}

	sched := NewScheduler(
		fakeFarms{farms: []*domain.Farm{farm1, farm2}},
		fakeAlerts{byFarm: map[uuid.UUID][]*domain.Weather{
			farm1.ID: {{ID: uuid.New(), FarmID: farm1.ID, Metric: "temperature", Operator: ">", Value: 35, IsEnabled: true}},
			farm2.ID: {{ID: uuid.New(), FarmID: farm2.ID, Metric: "temperature", Operator: ">", Value: 40, IsEnabled: true}},
		}},
		fakeUsers{byID: map[uuid.UUID]*domain.User{
			userID: {ID: userID, Email: "farmer@example.com"},
		}},
		fp,
		pub,
	)

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
}
