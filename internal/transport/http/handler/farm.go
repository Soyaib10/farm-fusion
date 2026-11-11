package handler

import (
	"context"
	"net/http"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/Soyaib10/farm-fusion/internal/platform/logger"
	farmsvc "github.com/Soyaib10/farm-fusion/internal/service/farm"
	"github.com/Soyaib10/farm-fusion/internal/transport/http/middleware"
	"github.com/Soyaib10/farm-fusion/internal/transport/http/response"
	"github.com/Soyaib10/farm-fusion/internal/validator"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// FarmService defines farm operations required by FarmHandler.
type FarmService interface {
	Create(ctx context.Context, userID uuid.UUID, input farmsvc.UpsertInput) (*domain.Farm, error)
	GetByID(ctx context.Context, farmID, userID uuid.UUID) (*domain.Farm, error)
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.Farm, error)
	Update(ctx context.Context, farmID, userID uuid.UUID, input farmsvc.UpsertInput) (*domain.Farm, error)
	Delete(ctx context.Context, farmID, userID uuid.UUID) error
}

// FarmHandler handles HTTP requests for farm CRUD operations.
type FarmHandler struct {
	svc FarmService
	log *logger.Logger
}

// NewFarmHandler constructs a FarmHandler.
func NewFarmHandler(svc FarmService, log *logger.Logger) *FarmHandler {
	return &FarmHandler{svc: svc, log: log}
}

// RegisterRoutes mounts farm and nested weather-alert routes onto r.
func (h *FarmHandler) RegisterRoutes(r chi.Router, weatherHandler *WeatherHandler) {
	r.Post("/", h.Create)
	r.Get("/", h.List)

	r.Route("/{farmID}", func(r chi.Router) {
		r.Get("/", h.GetByID)
		r.Put("/", h.Update)
		r.Delete("/", h.Delete)

		if weatherHandler != nil {
			r.Route("/weather-alerts", weatherHandler.RegisterRoutes)
		}
	})
}

type upsertFarmRequest struct {
	Name      string   `json:"name"`
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
}

type farmResponse struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Latitude    float64   `json:"latitude"`
	Longitude   float64   `json:"longitude"`
	LocationKey string    `json:"location_key"`
}

func toFarmResponse(f *domain.Farm) farmResponse {
	return farmResponse{
		ID:          f.ID,
		Name:        f.Name,
		Latitude:    f.Latitude,
		Longitude:   f.Longitude,
		LocationKey: f.LocationKey,
	}
}

func decodeUpsertFarm(w http.ResponseWriter, r *http.Request) (farmsvc.UpsertInput, error) {
	var req upsertFarmRequest
	if err := response.ReadJSON(w, r, &req); err != nil {
		return farmsvc.UpsertInput{}, err
	}

	v := validator.New()
	v.Check(req.Latitude != nil, "latitude", "must be provided")
	v.Check(req.Longitude != nil, "longitude", "must be provided")
	if err := v.Err(); err != nil {
		return farmsvc.UpsertInput{}, err
	}

	return farmsvc.UpsertInput{
		Name:      req.Name,
		Latitude:  *req.Latitude,
		Longitude: *req.Longitude,
	}, nil
}

// Create handles POST /api/v1/farms.
func (h *FarmHandler) Create(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.ActorFromContext(r.Context())
	if !ok {
		response.UnauthorizedBearer(w, r, h.log, "invalid or missing authentication token")
		return
	}

	input, err := decodeUpsertFarm(w, r)
	if err != nil {
		if _, isVal := err.(*domain.ValidationError); isVal {
			response.HandleDomainError(w, r, h.log, err)
		} else {
			response.BadRequest(w, r, h.log, err)
		}
		return
	}

	created, err := h.svc.Create(r.Context(), actor.ID, input)
	if err != nil {
		response.HandleDomainError(w, r, h.log, err)
		return
	}

	if err := response.WriteJSON(w, http.StatusCreated, response.Envelope{"farm": toFarmResponse(created)}, nil); err != nil {
		response.ServerError(w, r, h.log, err)
	}
}

// GetByID handles GET /api/v1/farms/{farmID}.
func (h *FarmHandler) GetByID(w http.ResponseWriter, r *http.Request) {
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

	f, err := h.svc.GetByID(r.Context(), farmID, actor.ID)
	if err != nil {
		response.HandleDomainError(w, r, h.log, err)
		return
	}

	if err := response.WriteJSON(w, http.StatusOK, response.Envelope{"farm": toFarmResponse(f)}, nil); err != nil {
		response.ServerError(w, r, h.log, err)
	}
}

// List handles GET /api/v1/farms.
func (h *FarmHandler) List(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.ActorFromContext(r.Context())
	if !ok {
		response.UnauthorizedBearer(w, r, h.log, "invalid or missing authentication token")
		return
	}

	farms, err := h.svc.ListByUserID(r.Context(), actor.ID)
	if err != nil {
		response.HandleDomainError(w, r, h.log, err)
		return
	}

	items := make([]farmResponse, len(farms))
	for i, f := range farms {
		items[i] = toFarmResponse(f)
	}

	if err := response.WriteJSON(w, http.StatusOK, response.Envelope{"farms": items}, nil); err != nil {
		response.ServerError(w, r, h.log, err)
	}
}

// Update handles PUT /api/v1/farms/{farmID}.
func (h *FarmHandler) Update(w http.ResponseWriter, r *http.Request) {
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

	input, err := decodeUpsertFarm(w, r)
	if err != nil {
		if _, isVal := err.(*domain.ValidationError); isVal {
			response.HandleDomainError(w, r, h.log, err)
		} else {
			response.BadRequest(w, r, h.log, err)
		}
		return
	}

	updated, err := h.svc.Update(r.Context(), farmID, actor.ID, input)
	if err != nil {
		response.HandleDomainError(w, r, h.log, err)
		return
	}

	if err := response.WriteJSON(w, http.StatusOK, response.Envelope{"farm": toFarmResponse(updated)}, nil); err != nil {
		response.ServerError(w, r, h.log, err)
	}
}

// Delete handles DELETE /api/v1/farms/{farmID}.
func (h *FarmHandler) Delete(w http.ResponseWriter, r *http.Request) {
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

	if err := h.svc.Delete(r.Context(), farmID, actor.ID); err != nil {
		response.HandleDomainError(w, r, h.log, err)
		return
	}

	if err := response.WriteJSON(w, http.StatusOK, response.Envelope{"message": "farm deleted successfully"}, nil); err != nil {
		response.ServerError(w, r, h.log, err)
	}
}
