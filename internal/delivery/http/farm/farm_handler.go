package farm

import (
	"errors"
	"net/http"

	"github.com/Soyaib10/farm-fusion/internal/app"
	"github.com/Soyaib10/farm-fusion/internal/delivery/http/middleware"
	"github.com/Soyaib10/farm-fusion/internal/usecase/farm"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	app     *app.Application
	usecase farm.UseCase
}

func NewHandler(app *app.Application, usecase farm.UseCase) *Handler {
	return &Handler{
		app:     app,
		usecase: usecase,
	}
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var input CreateFarmJSON
	if err := h.app.ReadJSON(w, r, &input); err != nil {
		h.app.BadRequestResponse(w, r, err)
		return
	}

	userID, err := middleware.GetUserIDFromContext(r)
	if err != nil {
		h.app.ServerErrorResponse(w, r, err)
		return
	}

	cmd := farm.UpsertFarmCommand{
		Name:      &input.Name,
		Longitude: &input.Longitude,
		Latitude:  &input.Latitude,
	}

	newFarm, err := h.usecase.Create(r.Context(), userID, cmd)
	if err != nil {
		h.app.ServerErrorResponse(w, r, err)
		return
	}

	response := FarmResponse{
		ID:          newFarm.ID,
		Name:        newFarm.Name,
		Latitude:    newFarm.Latitude,
		Longitude:   newFarm.Longitude,
		LocationKey: newFarm.LocationKey,
	}

	if err := h.app.WriteJSON(w, http.StatusCreated, app.Envelope{"farm": response}, nil); err != nil {
		h.app.ServerErrorResponse(w, r, err)
	}
}
func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	farmIDStr := chi.URLParam(r, "farmID")
	farmID, err := uuid.Parse(farmIDStr)
	if err != nil {
		h.app.BadRequestResponse(w, r, errors.New("invalid farm ID format"))
		return
	}

	userID, err := middleware.GetUserIDFromContext(r)
	if err != nil {
		h.app.ServerErrorResponse(w, r, err)
		return
	}

	farm, err := h.usecase.GetByID(r.Context(), farmID, userID)
	if err != nil {
		h.app.ServerErrorResponse(w, r, err)
		return
	}

	response := FarmResponse{
		ID:          farm.ID,
		Name:        farm.Name,
		Latitude:    farm.Latitude,
		Longitude:   farm.Longitude,
		LocationKey: farm.LocationKey,
	}

	if err := h.app.WriteJSON(w, http.StatusOK, app.Envelope{"farm": response}, nil); err != nil {
		h.app.ServerErrorResponse(w, r, err)
	}
}

func (h *Handler) ListByUserID(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserIDFromContext(r)
	if err != nil {
		h.app.ServerErrorResponse(w, r, err)
		return
	}

	farms, err := h.usecase.ListByUserID(r.Context(), userID)
	if err != nil {
		h.app.ServerErrorResponse(w, r, err)
		return
	}

	farmResponses := make([]FarmResponse, len(farms))
	for i, farm := range farms {
		farmResponses[i] = FarmResponse{
			ID:          farm.ID,
			Name:        farm.Name,
			Latitude:    farm.Latitude,
			Longitude:   farm.Longitude,
			LocationKey: farm.LocationKey,
		}
	}

	if err := h.app.WriteJSON(w, http.StatusOK, app.Envelope{"farms": farmResponses}, nil); err != nil {
		h.app.ServerErrorResponse(w, r, err)
	}
}
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	farmIDStr := chi.URLParam(r, "farmID")
	farmID, err := uuid.Parse(farmIDStr)
	if err != nil {
		h.app.BadRequestResponse(w, r, errors.New("invalid farm ID format"))
		return
	}

	var input UpdateFarmJSON
	if err := h.app.ReadJSON(w, r, &input); err != nil {
		h.app.BadRequestResponse(w, r, err)
		return
	}

	userID, err := middleware.GetUserIDFromContext(r)
	if err != nil {
		h.app.ServerErrorResponse(w, r, err)
		return
	}

	cmd := farm.UpsertFarmCommand{
		Name:      &input.Name,
		Longitude: &input.Longitude,
		Latitude:  &input.Latitude,
	}

	updatedFarm, err := h.usecase.Update(r.Context(), farmID, userID, cmd)
	if err != nil {
		h.app.ServerErrorResponse(w, r, err)
		return
	}

	response := FarmResponse{
		ID:          updatedFarm.ID,
		Name:        updatedFarm.Name,
		Latitude:    updatedFarm.Latitude,
		Longitude:   updatedFarm.Longitude,
		LocationKey: updatedFarm.LocationKey,
	}

	if err := h.app.WriteJSON(w, http.StatusOK, app.Envelope{"farm": response}, nil); err != nil {
		h.app.ServerErrorResponse(w, r, err)
	}
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	farmIDStr := chi.URLParam(r, "farmID")
	farmID, err := uuid.Parse(farmIDStr)
	if err != nil {
		h.app.BadRequestResponse(w, r, errors.New("invalid farm ID format"))
		return
	}

	userID, err := middleware.GetUserIDFromContext(r)
	if err != nil {
		h.app.ServerErrorResponse(w, r, err)
		return
	}

	if err := h.usecase.Delete(r.Context(), farmID, userID); err != nil {
		h.app.ServerErrorResponse(w, r, err)
		return
	}

	if err := h.app.WriteJSON(w, http.StatusOK, app.Envelope{"message": "farm deleted successfully"}, nil); err != nil {
		h.app.ServerErrorResponse(w, r, err)
	}
}
