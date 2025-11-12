package farm

import (
	"encoding/json"
	"net/http"

	"github.com/Soyaib10/farm-fusion/internal/usecase/farm"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	usecase farm.UseCase
}

func NewHandler(usecase farm.UseCase) *Handler {
	return &Handler{
		usecase: usecase,
	}
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateFarmRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request payload", http.StatusBadRequest)
		return
	}

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		http.Error(w, "invalid user ID", http.StatusBadRequest)
		return
	}

	farm, err := h.usecase.Create(r.Context(), userID, req.Name, req.Latitude, req.Longitude, req.SoilType)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := FarmResponse{
		ID:        farm.ID.String(),
		UserID:    farm.UserID.String(),
		Name:      farm.Name,
		Latitude:  farm.Latitude,
		Longitude: farm.Longitude,
		SoilType:  farm.SoilType,
		CreatedAt: farm.CreatedAt,
		UpdatedAt: farm.UpdatedAt,
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "invalid farm ID", http.StatusBadRequest)
		return
	}

	farm, err := h.usecase.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "farm not found", http.StatusNotFound)
		return
	}

	resp := FarmResponse{
		ID:        farm.ID.String(),
		UserID:    farm.UserID.String(),
		Name:      farm.Name,
		Latitude:  farm.Latitude,
		Longitude: farm.Longitude,
		SoilType:  farm.SoilType,
		CreatedAt: farm.CreatedAt,
		UpdatedAt: farm.UpdatedAt,
	}

	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	var req UpdateFarmRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request payload", http.StatusBadRequest)
		return
	}

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		http.Error(w, "invalid user ID", http.StatusBadRequest)
		return
	}

	farm, err := h.usecase.Update(r.Context(), userID, req.Name, req.Latitude, req.Longitude, req.SoilType)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := FarmResponse{
		UserID:    farm.UserID.String(),
		Name:      farm.Name,
		Latitude:  farm.Latitude,
		Longitude: farm.Longitude,
		SoilType:  farm.SoilType,
		CreatedAt: farm.CreatedAt,
		UpdatedAt: farm.UpdatedAt,
	}

	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "invalid farm ID", http.StatusBadRequest)
		return
	}

	err = h.usecase.Delete(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListByUser(w http.ResponseWriter, r *http.Request) {
	userIDStr := chi.URLParam(r, "userID")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		http.Error(w, "invalid user ID", http.StatusBadRequest)
		return
	}

	farms, err := h.usecase.ListByUser(r.Context(), userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := make([]FarmResponse, len(farms))
	for i, farm := range farms {
		resp[i] = FarmResponse{
			ID:        farm.ID.String(),
			UserID:    farm.UserID.String(),
			Name:      farm.Name,
			Latitude:  farm.Latitude,
			Longitude: farm.Longitude,
			SoilType:  farm.SoilType,
			CreatedAt: farm.CreatedAt,
			UpdatedAt: farm.UpdatedAt,
		}
	}

	json.NewEncoder(w).Encode(resp)
}
