package handler

import (
	"context"
	"net/http"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/Soyaib10/farm-fusion/internal/platform/logger"
	usersvc "github.com/Soyaib10/farm-fusion/internal/service/user"
	"github.com/Soyaib10/farm-fusion/internal/transport/http/middleware"
	"github.com/Soyaib10/farm-fusion/internal/transport/http/response"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// UserService defines user operations required by UserHandler.
type UserService interface {
	GetByID(ctx context.Context, actor domain.Actor, targetID uuid.UUID) (*domain.User, error)
	Update(ctx context.Context, actor domain.Actor, targetID uuid.UUID, input usersvc.UpdateInput) (*domain.User, error)
	Delete(ctx context.Context, actor domain.Actor, targetID uuid.UUID) error
	List(ctx context.Context, actor domain.Actor) ([]*domain.User, error)
}

// UserHandler handles HTTP requests for user profile management.
type UserHandler struct {
	svc UserService
	log *logger.Logger
}

// NewUserHandler constructs a UserHandler.
func NewUserHandler(svc UserService, log *logger.Logger) *UserHandler {
	return &UserHandler{svc: svc, log: log}
}

// RegisterRoutes mounts user endpoints onto r.
func (h *UserHandler) RegisterRoutes(r chi.Router) {
	r.Get("/me", h.GetMe)
	r.Patch("/me", h.UpdateMe)
	r.Delete("/me", h.DeleteMe)

	r.With(middleware.RequireAdmin(h.log)).Get("/", h.List)

	r.Get("/{id}", h.GetByID)
	r.Patch("/{id}", h.Update)
	r.Delete("/{id}", h.Delete)
}

type updateUserRequest struct {
	Name  *string `json:"name,omitempty"`
	Email *string `json:"email,omitempty"`
}

// GetMe handles GET /api/v1/users/me.
func (h *UserHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.ActorFromContext(r.Context())
	if !ok {
		response.UnauthorizedBearer(w, r, h.log, "invalid or missing authentication token")
		return
	}

	u, err := h.svc.GetByID(r.Context(), actor, actor.ID)
	if err != nil {
		response.HandleDomainError(w, r, h.log, err)
		return
	}

	if err := response.WriteJSON(w, http.StatusOK, response.Envelope{"user": u}, nil); err != nil {
		response.ServerError(w, r, h.log, err)
	}
}

// UpdateMe handles PATCH /api/v1/users/me.
func (h *UserHandler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.ActorFromContext(r.Context())
	if !ok {
		response.UnauthorizedBearer(w, r, h.log, "invalid or missing authentication token")
		return
	}
	h.updateTarget(w, r, actor, actor.ID)
}

// DeleteMe handles DELETE /api/v1/users/me.
func (h *UserHandler) DeleteMe(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.ActorFromContext(r.Context())
	if !ok {
		response.UnauthorizedBearer(w, r, h.log, "invalid or missing authentication token")
		return
	}
	h.deleteTarget(w, r, actor, actor.ID)
}

// GetByID handles GET /api/v1/users/{id}.
func (h *UserHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.ActorFromContext(r.Context())
	if !ok {
		response.UnauthorizedBearer(w, r, h.log, "invalid or missing authentication token")
		return
	}

	id, err := response.ReadUUIDParam(r, "id")
	if err != nil {
		response.BadRequest(w, r, h.log, err)
		return
	}

	u, err := h.svc.GetByID(r.Context(), actor, id)
	if err != nil {
		response.HandleDomainError(w, r, h.log, err)
		return
	}

	if err := response.WriteJSON(w, http.StatusOK, response.Envelope{"user": u}, nil); err != nil {
		response.ServerError(w, r, h.log, err)
	}
}

// Update handles PATCH /api/v1/users/{id}.
func (h *UserHandler) Update(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.ActorFromContext(r.Context())
	if !ok {
		response.UnauthorizedBearer(w, r, h.log, "invalid or missing authentication token")
		return
	}

	id, err := response.ReadUUIDParam(r, "id")
	if err != nil {
		response.BadRequest(w, r, h.log, err)
		return
	}
	h.updateTarget(w, r, actor, id)
}

func (h *UserHandler) updateTarget(w http.ResponseWriter, r *http.Request, actor domain.Actor, targetID uuid.UUID) {
	var req updateUserRequest
	if err := response.ReadJSON(w, r, &req); err != nil {
		response.BadRequest(w, r, h.log, err)
		return
	}

	updated, err := h.svc.Update(r.Context(), actor, targetID, usersvc.UpdateInput{
		Name:  req.Name,
		Email: req.Email,
	})
	if err != nil {
		response.HandleDomainError(w, r, h.log, err)
		return
	}

	if err := response.WriteJSON(w, http.StatusOK, response.Envelope{"user": updated}, nil); err != nil {
		response.ServerError(w, r, h.log, err)
	}
}

// Delete handles DELETE /api/v1/users/{id}.
func (h *UserHandler) Delete(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.ActorFromContext(r.Context())
	if !ok {
		response.UnauthorizedBearer(w, r, h.log, "invalid or missing authentication token")
		return
	}

	id, err := response.ReadUUIDParam(r, "id")
	if err != nil {
		response.BadRequest(w, r, h.log, err)
		return
	}
	h.deleteTarget(w, r, actor, id)
}

func (h *UserHandler) deleteTarget(w http.ResponseWriter, r *http.Request, actor domain.Actor, targetID uuid.UUID) {
	if err := h.svc.Delete(r.Context(), actor, targetID); err != nil {
		response.HandleDomainError(w, r, h.log, err)
		return
	}

	if err := response.WriteJSON(w, http.StatusOK, response.Envelope{"message": "user successfully deleted"}, nil); err != nil {
		response.ServerError(w, r, h.log, err)
	}
}

// List handles GET /api/v1/users (admin only).
func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.ActorFromContext(r.Context())
	if !ok {
		response.UnauthorizedBearer(w, r, h.log, "invalid or missing authentication token")
		return
	}

	users, err := h.svc.List(r.Context(), actor)
	if err != nil {
		response.HandleDomainError(w, r, h.log, err)
		return
	}
	if users == nil {
		users = []*domain.User{}
	}

	if err := response.WriteJSON(w, http.StatusOK, response.Envelope{"users": users}, nil); err != nil {
		response.ServerError(w, r, h.log, err)
	}
}
