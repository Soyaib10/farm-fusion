package user

import (
	"encoding/json"
	"net/http"

	"github.com/Soyaib10/farm-fusion/internal/usecase/user"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	usecase user.UseCase
}

func NewHandler(usecase user.UseCase) *Handler {
	return &Handler{
		usecase: usecase,
	}
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request payload", http.StatusBadRequest)
		return
	}

	// use validation logic here

	user, err := h.usecase.Create(r.Context(), req.Name, req.Email)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := CreateUserResponse{
		ID:    user.ID.String(),
		Name:  user.Name,
		Email: user.Email,
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "invalid user ID", http.StatusBadRequest)
		return
	}

	user, err := h.usecase.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	resp := GetUserResponse{
		ID:    user.ID.String(),
		Name:  user.Name,
		Email: user.Email,
	}

	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "invalid user ID", http.StatusBadRequest)
		return
	}

	var req UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request payload", http.StatusBadRequest)
		return
	}

	user, err := h.usecase.Update(r.Context(), id, req.Name, req.Email)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := UpdateUserResponse{
		ID:    user.ID.String(),
		Name:  user.Name,
		Email: user.Email,
	}

	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "invalid user ID", http.StatusBadRequest)
		return
	}

	if err := h.usecase.Delete(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Find(w http.ResponseWriter, r *http.Request) {
	users, err := h.usecase.Find(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var resp []FindUserResponse
	for _, user := range users {
		resp = append(resp, FindUserResponse{
			ID:    user.ID.String(),
			Name:  user.Name,
			Email: user.Email,
		})
	}

	json.NewEncoder(w).Encode(resp)
}