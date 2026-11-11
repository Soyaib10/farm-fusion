package handler

import (
	"context"
	"net/http"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/Soyaib10/farm-fusion/internal/platform/logger"
	weathersvc "github.com/Soyaib10/farm-fusion/internal/service/weather"
	"github.com/Soyaib10/farm-fusion/internal/transport/http/middleware"
	"github.com/Soyaib10/farm-fusion/internal/transport/http/response"
	"github.com/Soyaib10/farm-fusion/internal/validator"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// WeatherService defines weather alert operations required by WeatherHandler.
type WeatherService interface {
	Create(ctx context.Context, input weathersvc.CreateInput) (*domain.Weather, error)
	GetByID(ctx context.Context, userID, farmID, weatherID uuid.UUID) (*domain.Weather, error)
	ListByFarm(ctx context.Context, userID, farmID uuid.UUID) ([]*domain.Weather, error)
	Update(ctx context.Context, input weathersvc.UpdateInput) (*domain.Weather, error)
	Delete(ctx context.Context, userID, farmID, weatherID uuid.UUID) error
}

// WeatherHandler handles HTTP requests for per-farm weather alert thresholds.
type WeatherHandler struct {
	svc WeatherService
	log *logger.Logger
}

// NewWeatherHandler constructs a WeatherHandler.
func NewWeatherHandler(svc WeatherService, log *logger.Logger) *WeatherHandler {
	return &WeatherHandler{svc: svc, log: log}
}

// RegisterRoutes mounts weather alert endpoints onto r.
func (h *WeatherHandler) RegisterRoutes(r chi.Router) {
	r.Post("/", h.Create)
	r.Get("/", h.List)
	r.Get("/{weatherID}", h.GetByID)
	r.Put("/{weatherID}", h.Update)
	r.Delete("/{weatherID}", h.Delete)
}

type createWeatherRequest struct {
	Metric   string   `json:"metric"`
	Operator string   `json:"operator"`
	Value    *float64 `json:"value"`
}

type updateWeatherRequest struct {
	Metric    string   `json:"metric"`
	Operator  string   `json:"operator"`
	Value     *float64 `json:"value"`
	IsEnabled bool     `json:"is_enabled"`
}

type weatherResponse struct {
	ID        uuid.UUID `json:"id"`
	FarmID    uuid.UUID `json:"farm_id"`
	Metric    string    `json:"metric"`
	Operator  string    `json:"operator"`
	Value     float64   `json:"value"`
	IsEnabled bool      `json:"is_enabled"`
}

func toWeatherResponse(w *domain.Weather) weatherResponse {
	return weatherResponse{
		ID:        w.ID,
		FarmID:    w.FarmID,
		Metric:    w.Metric,
		Operator:  w.Operator,
		Value:     w.Value,
		IsEnabled: w.IsEnabled,
	}
}

// Create handles POST /api/v1/farms/{farmID}/weather-alerts.
func (h *WeatherHandler) Create(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.ActorFromContext(r.Context())
	if !ok {
		response.UnauthorizedBearer(w, r, h.log, "invalid or missing authentication token")
		return
	}

	farmID, err := response.ReadUUIDParam(r, "farmID")
	if err != nil {
		response.BadRequest(w, r, h.log, err)
		return
	}

	var req createWeatherRequest
	if err := response.ReadJSON(w, r, &req); err != nil {
		response.BadRequest(w, r, h.log, err)
		return
	}

	v := validator.New()
	v.Check(req.Value != nil, "value", "must be provided")
	if err := v.Err(); err != nil {
		response.HandleDomainError(w, r, h.log, err)
		return
	}

	created, err := h.svc.Create(r.Context(), weathersvc.CreateInput{
		UserID:   actor.ID,
		FarmID:   farmID,
		Metric:   req.Metric,
		Operator: req.Operator,
		Value:    *req.Value,
	})
	if err != nil {
		response.HandleDomainError(w, r, h.log, err)
		return
	}

	if err := response.WriteJSON(w, http.StatusCreated, response.Envelope{"weather": toWeatherResponse(created)}, nil); err != nil {
		response.ServerError(w, r, h.log, err)
	}
}

// GetByID handles GET /api/v1/farms/{farmID}/weather-alerts/{weatherID}.
func (h *WeatherHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.ActorFromContext(r.Context())
	if !ok {
		response.UnauthorizedBearer(w, r, h.log, "invalid or missing authentication token")
		return
	}

	farmID, err := response.ReadUUIDParam(r, "farmID")
	if err != nil {
		response.BadRequest(w, r, h.log, err)
		return
	}
	weatherID, err := response.ReadUUIDParam(r, "weatherID")
	if err != nil {
		response.BadRequest(w, r, h.log, err)
		return
	}

	rule, err := h.svc.GetByID(r.Context(), actor.ID, farmID, weatherID)
	if err != nil {
		response.HandleDomainError(w, r, h.log, err)
		return
	}

	if err := response.WriteJSON(w, http.StatusOK, response.Envelope{"weather": toWeatherResponse(rule)}, nil); err != nil {
		response.ServerError(w, r, h.log, err)
	}
}

// List handles GET /api/v1/farms/{farmID}/weather-alerts.
func (h *WeatherHandler) List(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.ActorFromContext(r.Context())
	if !ok {
		response.UnauthorizedBearer(w, r, h.log, "invalid or missing authentication token")
		return
	}

	farmID, err := response.ReadUUIDParam(r, "farmID")
	if err != nil {
		response.BadRequest(w, r, h.log, err)
		return
	}

	rules, err := h.svc.ListByFarm(r.Context(), actor.ID, farmID)
	if err != nil {
		response.HandleDomainError(w, r, h.log, err)
		return
	}

	items := make([]weatherResponse, len(rules))
	for i, rule := range rules {
		items[i] = toWeatherResponse(rule)
	}

	if err := response.WriteJSON(w, http.StatusOK, response.Envelope{"weathers": items}, nil); err != nil {
		response.ServerError(w, r, h.log, err)
	}
}

// Update handles PUT /api/v1/farms/{farmID}/weather-alerts/{weatherID}.
func (h *WeatherHandler) Update(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.ActorFromContext(r.Context())
	if !ok {
		response.UnauthorizedBearer(w, r, h.log, "invalid or missing authentication token")
		return
	}

	farmID, err := response.ReadUUIDParam(r, "farmID")
	if err != nil {
		response.BadRequest(w, r, h.log, err)
		return
	}
	weatherID, err := response.ReadUUIDParam(r, "weatherID")
	if err != nil {
		response.BadRequest(w, r, h.log, err)
		return
	}

	var req updateWeatherRequest
	if err := response.ReadJSON(w, r, &req); err != nil {
		response.BadRequest(w, r, h.log, err)
		return
	}

	v := validator.New()
	v.Check(req.Value != nil, "value", "must be provided")
	if err := v.Err(); err != nil {
		response.HandleDomainError(w, r, h.log, err)
		return
	}

	updated, err := h.svc.Update(r.Context(), weathersvc.UpdateInput{
		UserID:    actor.ID,
		FarmID:    farmID,
		WeatherID: weatherID,
		Metric:    req.Metric,
		Operator:  req.Operator,
		Value:     *req.Value,
		IsEnabled: req.IsEnabled,
	})
	if err != nil {
		response.HandleDomainError(w, r, h.log, err)
		return
	}

	if err := response.WriteJSON(w, http.StatusOK, response.Envelope{"weather": toWeatherResponse(updated)}, nil); err != nil {
		response.ServerError(w, r, h.log, err)
	}
}

// Delete handles DELETE /api/v1/farms/{farmID}/weather-alerts/{weatherID}.
func (h *WeatherHandler) Delete(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.ActorFromContext(r.Context())
	if !ok {
		response.UnauthorizedBearer(w, r, h.log, "invalid or missing authentication token")
		return
	}

	farmID, err := response.ReadUUIDParam(r, "farmID")
	if err != nil {
		response.BadRequest(w, r, h.log, err)
		return
	}
	weatherID, err := response.ReadUUIDParam(r, "weatherID")
	if err != nil {
		response.BadRequest(w, r, h.log, err)
		return
	}

	if err := h.svc.Delete(r.Context(), actor.ID, farmID, weatherID); err != nil {
		response.HandleDomainError(w, r, h.log, err)
		return
	}

	if err := response.WriteJSON(w, http.StatusOK, response.Envelope{"message": "weather deleted successfully"}, nil); err != nil {
		response.ServerError(w, r, h.log, err)
	}
}
