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

type SchedulerUseCase interface {
	RunScheduled(ctx context.Context) (*RunResult, error)
}

type RunResult struct {
	FarmsScanned           int      `json:"farms_scanned"`
	ForecastsFetched       int      `json:"forecasts_fetched"`
	NotificationsPublished int      `json:"notifications_published"`
	Errors                 []string `json:"errors,omitempty"`
}

type schedulerUseCase struct {
	farms     FarmRepository
	alerts    WeatherAlertRepository
	users     UserRepository
	forecasts ForecastProvider
	publisher QueuePublisher
	now       func() time.Time
}

func NewSchedulerUseCase(farms FarmRepository, alerts WeatherAlertRepository, users UserRepository, forecasts ForecastProvider, publisher QueuePublisher) SchedulerUseCase {
	return &schedulerUseCase{
		farms:     farms,
		alerts:    alerts,
		users:     users,
		forecasts: forecasts,
		publisher: publisher,
		now:       func() time.Time { return time.Now().UTC() },
	}
}

func (uc *schedulerUseCase) RunScheduled(ctx context.Context) (*RunResult, error) {
	farms, err := uc.farms.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("list farms: %w", err)
	}

	result := &RunResult{}
	forecastByLocation := make(map[string]*domain.WeatherForecast)

	for _, farm := range farms {
		result.FarmsScanned++

		locationKey := farm.LocationKey
		if locationKey == "" {
			locationKey = domain.NewLocationKey(farm.Latitude, farm.Longitude)
		}

		forecast, ok := forecastByLocation[locationKey]
		if !ok {
			forecast, err = uc.forecasts.FetchForecast(ctx, farm.Latitude, farm.Longitude, locationKey)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("farm %s forecast: %v", farm.ID, err))
				continue
			}
			forecastByLocation[locationKey] = forecast
			result.ForecastsFetched++
		}

		farmAlerts, err := uc.alerts.ListByFarmID(ctx, farm.ID)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("farm %s alerts: %v", farm.ID, err))
			continue
		}

		weatherAlerts := buildWeatherAlerts(farmAlerts, forecast)
		if len(weatherAlerts) == 0 {
			continue
		}

		user, err := uc.users.GetByID(ctx, farm.UserID)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("farm %s user %s: %v", farm.ID, farm.UserID, err))
			continue
		}

		payload := &domain.NotificationPayload{
			NotificationID: uuid.New(),
			FarmID:         farm.ID,
			UserID:         farm.UserID,
			UserEmail:      user.Email,
			FarmName:       farm.Name,
			Location: domain.Location{
				Latitude:  farm.Latitude,
				Longitude: farm.Longitude,
			},
			Timestamp:       uc.now(),
			Alerts:          weatherAlerts,
			ForecastSummary: summarizeForecast(forecast),
		}

		if err := uc.publisher.Publish(ctx, payload); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("farm %s publish: %v", farm.ID, err))
			continue
		}
		result.NotificationsPublished++
	}

	return result, nil
}

func buildWeatherAlerts(thresholds []*domain.Weather, forecast *domain.WeatherForecast) []domain.WeatherAlert {
	alerts := make([]domain.WeatherAlert, 0, len(thresholds))
	for _, threshold := range thresholds {
		if !threshold.IsEnabled {
			continue
		}

		ranges := matchingRanges(threshold, forecast.DataPoints)
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

func matchingRanges(threshold *domain.Weather, points []domain.ForecastPoint) []domain.AlertRange {
	var ranges []domain.AlertRange
	var current []domain.ForecastPoint

	flush := func() {
		if len(current) == 0 {
			return
		}

		minValue := math.Inf(1)
		maxValue := math.Inf(-1)
		for _, point := range current {
			value, _ := metricValue(threshold.Metric, point)
			if value < minValue {
				minValue = value
			}
			if value > maxValue {
				maxValue = value
			}
		}

		start := current[0].Timestamp
		end := current[len(current)-1].Timestamp.Add(3 * time.Hour)
		ranges = append(ranges, domain.AlertRange{
			Start:         start,
			End:           end,
			DurationHours: int(end.Sub(start).Hours()),
			MinValue:      minValue,
			MaxValue:      maxValue,
			DataPoints:    append([]domain.ForecastPoint(nil), current...),
		})
		current = nil
	}

	for _, point := range points {
		value, ok := metricValue(threshold.Metric, point)
		if !ok {
			flush()
			continue
		}

		if compare(value, threshold.Operator, threshold.Value) {
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
		return actual == expected
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
