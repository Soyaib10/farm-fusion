package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/Soyaib10/farm-fusion/internal/platform/logger"
	authsvc "github.com/Soyaib10/farm-fusion/internal/service/auth"
	"github.com/Soyaib10/farm-fusion/internal/transport/http/middleware"
	"github.com/Soyaib10/farm-fusion/internal/transport/http/response"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// AuthService defines authentication operations required by AuthHandler.
type AuthService interface {
	Register(ctx context.Context, input authsvc.RegisterInput) error
	Login(ctx context.Context, input authsvc.LoginInput) (*authsvc.AuthResult, error)
	Refresh(ctx context.Context, rawRefreshToken string) (*authsvc.AuthResult, error)
	Logout(ctx context.Context, rawRefreshToken, rawAccessToken string) error
	LogoutAll(ctx context.Context, actor domain.Actor) error
}

// AuthHandler handles HTTP requests for user authentication and session management.
type AuthHandler struct {
	svc AuthService
	log *logger.Logger
}

// NewAuthHandler constructs an AuthHandler.
func NewAuthHandler(svc AuthService, log *logger.Logger) *AuthHandler {
	return &AuthHandler{svc: svc, log: log}
}

// RegisterRoutes mounts auth endpoints onto r. requireAuth guards the endpoints that need a session.
func (h *AuthHandler) RegisterRoutes(r chi.Router, requireAuth func(http.Handler) http.Handler) {
	r.Post("/register", h.Register)
	r.Post("/login", h.Login)
	r.Post("/refresh", h.Refresh)
	r.Post("/logout", h.Logout)
	r.With(requireAuth).Post("/logout-all", h.LogoutAll)
}

type registerRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type authUserResponse struct {
	ID        uuid.UUID   `json:"id"`
	Name      string      `json:"name"`
	Email     string      `json:"email"`
	Role      domain.Role `json:"role"`
	CreatedAt time.Time   `json:"created_at"`
}

type tokenPairResponse struct {
	AccessToken  string           `json:"access_token"`
	RefreshToken string           `json:"refresh_token"`
	User         authUserResponse `json:"user"`
}

func toAuthUserResponse(u *domain.User) authUserResponse {
	return authUserResponse{
		ID:        u.ID,
		Name:      u.Name,
		Email:     u.Email,
		Role:      u.Role,
		CreatedAt: u.CreatedAt,
	}
}

// Register handles POST /api/v1/auth/register.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := response.ReadJSON(w, r, &req); err != nil {
		response.BadRequest(w, r, h.log, err)
		return
	}

	if err := h.svc.Register(r.Context(), authsvc.RegisterInput{
		Name:     req.Name,
		Email:    req.Email,
		Password: req.Password,
	}); err != nil {
		response.HandleDomainError(w, r, h.log, err)
		return
	}

	// Same response whether or not the email was already registered.
	data := map[string]any{
		"message": "Registration received. If the email address is not already registered, you can now log in.",
	}
	if err := response.WriteJSON(w, http.StatusAccepted, response.Envelope{"data": data}, nil); err != nil {
		response.ServerError(w, r, h.log, err)
	}
}

// Login handles POST /api/v1/auth/login.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := response.ReadJSON(w, r, &req); err != nil {
		response.BadRequest(w, r, h.log, err)
		return
	}

	res, err := h.svc.Login(r.Context(), authsvc.LoginInput{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		response.HandleDomainError(w, r, h.log, err)
		return
	}

	payload := tokenPairResponse{
		AccessToken:  res.AccessToken,
		RefreshToken: res.RefreshToken,
		User:         toAuthUserResponse(res.User),
	}
	if err := response.WriteJSON(w, http.StatusOK, response.Envelope{"auth": payload}, nil); err != nil {
		response.ServerError(w, r, h.log, err)
	}
}

// Refresh handles POST /api/v1/auth/refresh.
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := response.ReadJSON(w, r, &req); err != nil {
		response.BadRequest(w, r, h.log, err)
		return
	}

	res, err := h.svc.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		response.HandleDomainError(w, r, h.log, err)
		return
	}

	payload := tokenPairResponse{
		AccessToken:  res.AccessToken,
		RefreshToken: res.RefreshToken,
		User:         toAuthUserResponse(res.User),
	}
	if err := response.WriteJSON(w, http.StatusOK, response.Envelope{"auth": payload}, nil); err != nil {
		response.ServerError(w, r, h.log, err)
	}
}

// Logout handles POST /api/v1/auth/logout.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := response.ReadJSON(w, r, &req); err != nil {
		response.BadRequest(w, r, h.log, err)
		return
	}

	accessToken, _ := middleware.BearerToken(r)
	if err := h.svc.Logout(r.Context(), req.RefreshToken, accessToken); err != nil {
		response.HandleDomainError(w, r, h.log, err)
		return
	}

	if err := response.WriteJSON(w, http.StatusOK, response.Envelope{"message": "logged out successfully"}, nil); err != nil {
		response.ServerError(w, r, h.log, err)
	}
}

// LogoutAll handles POST /api/v1/auth/logout-all, revoking every session of the caller.
func (h *AuthHandler) LogoutAll(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.ActorFromContext(r.Context())
	if !ok {
		response.UnauthorizedBearer(w, r, h.log, "invalid or missing authentication token")
		return
	}

	if err := h.svc.LogoutAll(r.Context(), actor); err != nil {
		response.HandleDomainError(w, r, h.log, err)
		return
	}

	if err := response.WriteJSON(w, http.StatusOK, response.Envelope{"message": "all sessions logged out"}, nil); err != nil {
		response.ServerError(w, r, h.log, err)
	}
}
