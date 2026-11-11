package middleware

import (
	"net/http"
	"strings"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/Soyaib10/farm-fusion/internal/platform/logger"
	"github.com/Soyaib10/farm-fusion/internal/transport/http/response"
)

// TokenVerifier verifies Bearer access tokens and returns the authenticated Actor.
type TokenVerifier interface {
	VerifyAccessToken(rawToken string) (domain.Actor, error)
}

// RequireAuth enforces a valid Bearer JWT access token on incoming requests.
func RequireAuth(verifier TokenVerifier, log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
			if authHeader == "" {
				response.UnauthorizedBearer(w, r, log, "invalid or missing authentication token")
				return
			}

			scheme, rawToken, found := strings.Cut(authHeader, " ")
			if !found || !strings.EqualFold(scheme, "Bearer") {
				response.UnauthorizedBearer(w, r, log, "invalid or missing authentication token")
				return
			}
			rawToken = strings.TrimSpace(rawToken)
			if rawToken == "" || len(rawToken) > 4096 {
				response.UnauthorizedBearer(w, r, log, "invalid or missing authentication token")
				return
			}

			actor, err := verifier.VerifyAccessToken(rawToken)
			if err != nil {
				response.UnauthorizedBearer(w, r, log, "invalid or expired authentication token")
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
