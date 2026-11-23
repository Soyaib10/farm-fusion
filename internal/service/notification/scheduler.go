package notification

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/google/uuid"
)

const (
	targetPageSize = 500
	// equalityTolerance is how close a forecast value must be to match an "=" rule.
	equalityTolerance = 1e-6
)

// TargetReader pages through farms with enabled alert rules, joined with owner email and rules.
type TargetReader interface {
	ListAlertTargets(ctx context.Context, after domain.AlertTargetCursor, limit int) ([]*domain.AlertTarget, error)
}

// Outbox reserves one notification per farm per day so reruns do not publish it twice.
type Outbox interface {
	// Reserve returns the notification ID for (farmID, alertDate) and whether it was already published.
	Reserve(ctx context.Context, farmID uuid.UUID, alertDate time.Time) (uuid.UUID, bool, error)
	MarkPublished(ctx context.Context, notificationID uuid.UUID) error
}

// ForecastProvider fetches 24-hour weather forecasts for a coordinate pair.
type ForecastProvider interface {
	FetchForecast(ctx context.Context, lat, lon float64, locationKey string) (*domain.WeatherForecast, error)
}

// Publisher enqueues weather notification payloads for asynchronous delivery.
type Publisher interface {
	Publish(ctx context.Context, payload *domain.NotificationPayload) error
}

// RunResult summarizes a scheduled weather evaluation pass.
type RunResult struct {
	FarmsScanned           int      `json:"farms_scanned"`
	ForecastsFetched       int      `json:"forecasts_fetched"`
	NotificationsPublished int      `json:"notifications_published"`
	AlreadyPublished       int      `json:"already_published"`
	Errors                 []string `json:"errors,omitempty"`
}

// Scheduler evaluates weather forecasts against farm alert thresholds and publishes notifications.
// Running it again on the same day is safe: farms already published for that day are skipped.
type Scheduler struct {
	targets   TargetReader
	forecasts ForecastProvider
	publisher Publisher
	outbox    Outbox
	location  *time.Location
	now       func() time.Time
}

// NewScheduler constructs a weather notification Scheduler. location defines the calendar day an
// alert belongs to and the zone timestamps are shown in.
func NewScheduler(
	targets TargetReader,
	forecasts ForecastProvider,
	publisher Publisher,
	outbox Outbox,
	location *time.Location,
) *Scheduler {
	if location == nil {
		location = time.UTC
	}
	return &Scheduler{
		targets:   targets,
		forecasts: forecasts,
		publisher: publisher,
		outbox:    outbox,
		location:  location,
		now:       time.Now,
	}
}

// RunScheduled executes one full weather check across all farms with enabled alert rules.
func (s *Scheduler) RunScheduled(ctx context.Context) (*RunResult, error) {
	result := &RunResult{}
	forecastByLocation := make(map[string]*domain.WeatherForecast)
	now := s.now().In(s.location)

	var cursor domain.AlertTargetCursor
	for {
		page, err := s.targets.ListAlertTargets(ctx, cursor, targetPageSize)
		if err != nil {
			return result, fmt.Errorf("list alert targets: %w", err)
		}

		for _, target := range page {
			if err := ctx.Err(); err != nil {
				return result, err
			}
			s.processTarget(ctx, target, now, forecastByLocation, result)
		}

		if len(page) < targetPageSize {
			return result, nil
		}
		last := page[len(page)-1].Farm
		cursor = domain.AlertTargetCursor{LocationKey: last.LocationKey, FarmID: last.ID}
	}
}

func (s *Scheduler) processTarget(
	ctx context.Context,
	target *domain.AlertTarget,
	now time.Time,
	forecastByLocation map[string]*domain.WeatherForecast,
	result *RunResult,
) {
	farm := target.Farm
	result.FarmsScanned++

	// Always derive the cache key in Go; stored keys may come from SQL rounding (migration 005).
	locationKey := domain.NewLocationKey(farm.Latitude, farm.Longitude)

	forecast, ok := forecastByLocation[locationKey]
	if !ok {
		var err error
		forecast, err = s.forecasts.FetchForecast(ctx, farm.Latitude, farm.Longitude, locationKey)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("farm %s forecast: %v", farm.ID, err))
			return
		}
		forecastByLocation[locationKey] = forecast
		result.ForecastsFetched++
	}

	weatherAlerts := buildWeatherAlerts(target.Alerts, forecast, s.location)
	if len(weatherAlerts) == 0 {
		return
	}

	notificationID, published, err := s.outbox.Reserve(ctx, farm.ID, now)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("farm %s reserve: %v", farm.ID, err))
		return
	}
	if published {
		result.AlreadyPublished++
		return
	}

	payload := &domain.NotificationPayload{
		NotificationID: notificationID,
		FarmID:         farm.ID,
		UserID:         farm.UserID,
		UserEmail:      target.OwnerEmail,
		FarmName:       farm.Name,
		Location: domain.Location{
			Latitude:  farm.Latitude,
			Longitude: farm.Longitude,
		},
		Timestamp:       now,
		Alerts:          weatherAlerts,
		ForecastSummary: summarizeForecast(forecast),
	}

	if err := s.publisher.Publish(ctx, payload); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("farm %s publish: %v", farm.ID, err))
		return
	}
	result.NotificationsPublished++

	if err := s.outbox.MarkPublished(ctx, notificationID); err != nil {
		// The message is on the queue. A rerun republishes the same ID, which the worker deduplicates.
		result.Errors = append(result.Errors, fmt.Sprintf("farm %s mark published: %v", farm.ID, err))
	}
}

func buildWeatherAlerts(thresholds []*domain.Weather, forecast *domain.WeatherForecast, loc *time.Location) []domain.WeatherAlert {
	alerts := make([]domain.WeatherAlert, 0, len(thresholds))
	for _, threshold := range thresholds {
		if !threshold.IsEnabled {
			continue
		}

		ranges := matchingRanges(threshold, forecast.DataPoints, loc)
		if len(ranges) == 0 {
			continue
		}

		alerts = append(alerts, domain.WeatherAlert{
			AlertType: normalizeMetric(threshold.Metric),
			Threshold: domain.Threshold{
				Operator: threshold.Operator,
				Value:    threshold.Value,
			},
			Ranges: ranges,
		})
	}
	return alerts
}

func matchingRanges(threshold *domain.Weather, points []domain.ForecastPoint, loc *time.Location) []domain.AlertRange {
	var (
		ranges  []domain.AlertRange
		current []domain.ForecastPoint
	)

	flush := func() {
		if len(current) == 0 {
			return
		}

		minVal := math.Inf(1)
		maxVal := math.Inf(-1)
		for _, p := range current {
			val, _ := metricValue(threshold.Metric, p)
			if val < minVal {
				minVal = val
			}
			if val > maxVal {
				maxVal = val
			}
		}

		start := current[0].Timestamp.In(loc)
		end := current[len(current)-1].Timestamp.Add(3 * time.Hour).In(loc)
		ranges = append(ranges, domain.AlertRange{
			Start:         start,
			End:           end,
			DurationHours: int(end.Sub(start).Hours()),
			MinValue:      minVal,
			MaxValue:      maxVal,
			DataPoints:    append([]domain.ForecastPoint(nil), current...),
		})
		current = nil
	}

	for _, point := range points {
		val, ok := metricValue(threshold.Metric, point)
		if !ok {
			flush()
			continue
		}
		if compare(val, threshold.Operator, threshold.Value) {
			current = append(current, point)
			continue
		}
		flush()
	}
	flush()

	return ranges
}

func metricValue(metric string, point domain.ForecastPoint) (float64, bool) {
	switch normalizeMetric(metric) {
	case "temperature", "temp":
		return point.Temperature, true
	case "humidity":
		return point.Humidity, true
	case "rainfall", "rain":
		return point.Rainfall, true
	case "wind_speed", "wind":
		return point.WindSpeed, true
	default:
		return 0, false
	}
}

func normalizeMetric(metric string) string {
	return strings.ToLower(strings.TrimSpace(metric))
}

func compare(actual float64, operator string, expected float64) bool {
	switch strings.TrimSpace(operator) {
	case "<":
		return actual < expected
	case "<=":
		return actual <= expected
	case ">":
		return actual > expected
	case ">=":
		return actual >= expected
	case "=", "==":
		// Forecast values are decoded JSON floats; 30.5 can arrive as 30.499999999.
		return math.Abs(actual-expected) <= equalityTolerance*math.Max(1, math.Abs(expected))
	default:
		return false
	}
}

func summarizeForecast(forecast *domain.WeatherForecast) domain.ForecastSummary {
	if len(forecast.DataPoints) == 0 {
		return domain.ForecastSummary{}
	}

	summary := domain.ForecastSummary{
		TempMin: forecast.DataPoints[0].Temperature,
		TempMax: forecast.DataPoints[0].Temperature,
	}
	for _, point := range forecast.DataPoints {
		if point.Temperature < summary.TempMin {
			summary.TempMin = point.Temperature
		}
		if point.Temperature > summary.TempMax {
			summary.TempMax = point.Temperature
		}
		summary.TotalRainfall += point.Rainfall
	}
	return summary
}
