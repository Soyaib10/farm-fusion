package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/Soyaib10/farm-fusion/internal/platform/logger"
	"github.com/Soyaib10/farm-fusion/internal/transport/http/response"
)

// Authenticator verifies Bearer access tokens against current account state and returns the Actor.
type Authenticator interface {
	Authenticate(ctx context.Context, rawToken string) (domain.Actor, error)
}

// BearerToken extracts the raw token from an "Authorization: Bearer <token>" header.
func BearerToken(r *http.Request) (string, bool) {
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if authHeader == "" {
		return "", false
	}
	scheme, rawToken, found := strings.Cut(authHeader, " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" || len(rawToken) > 4096 {
		return "", false
	}
	return rawToken, true
}

// RequireAuth enforces a valid Bearer JWT access token on incoming requests.
func RequireAuth(auth Authenticator, log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rawToken, ok := BearerToken(r)
			if !ok {
				response.UnauthorizedBearer(w, r, log, "invalid or missing authentication token")
				return
			}

			actor, err := auth.Authenticate(r.Context(), rawToken)
			if err != nil {
				if errors.Is(err, domain.ErrUnauthorized) {
					response.UnauthorizedBearer(w, r, log, "invalid or expired authentication token")
					return
				}
				response.ServerError(w, r, log, err)
				return
			}

			ctx := WithActor(r.Context(), actor)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireAdmin restricts access to authenticated actors with domain.RoleAdmin.
func RequireAdmin(log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actor, ok := ActorFromContext(r.Context())
			if !ok {
				response.UnauthorizedBearer(w, r, log, "invalid or missing authentication token")
				return
			}
			if !actor.IsAdmin() {
				response.Forbidden(w, r, log)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
