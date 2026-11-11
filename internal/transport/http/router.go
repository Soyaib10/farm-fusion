package httpapi

import (
	"net/http"

	"github.com/Soyaib10/farm-fusion/internal/config"
	"github.com/Soyaib10/farm-fusion/internal/platform/logger"
	"github.com/Soyaib10/farm-fusion/internal/transport/http/handler"
	"github.com/Soyaib10/farm-fusion/internal/transport/http/middleware"
	"github.com/Soyaib10/farm-fusion/internal/transport/http/response"
	"github.com/go-chi/chi/v5"
)

// Dependencies bundles handlers and security components wired into the HTTP router.
type Dependencies struct {
	Config         *config.Config
	Logger         *logger.Logger
	TokenVerifier  middleware.TokenVerifier
	Health         *handler.HealthHandler
	Auth           *handler.AuthHandler
	User           *handler.UserHandler
	Farm           *handler.FarmHandler
	Weather        *handler.WeatherHandler
	Recommendation *handler.RecommendationHandler
	GlobalLimiter  *middleware.RateLimiter
	AuthLimiter    *middleware.RateLimiter
}

// NewRouter constructs a hardened Chi HTTP router with security headers, CORS, rate limiting,
// panic recovery, structured request logging, and JWT authorization.
func NewRouter(deps Dependencies) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer(deps.Logger))
	r.Use(middleware.SecurityHeaders(deps.Config.IsProduction()))
	r.Use(middleware.CORS(deps.Config.CORS.AllowedOrigins))
	r.Use(middleware.RequestLogger(deps.Logger))

	if deps.GlobalLimiter != nil {
		r.Use(deps.GlobalLimiter.Middleware)
	}

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		response.NotFound(w, r, deps.Logger)
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		response.MethodNotAllowed(w, r, deps.Logger)
	})

	if deps.Health != nil {
		r.Get("/healthz", deps.Health.Live)
		r.Get("/readyz", deps.Health.Ready)
	}

	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			r.Use(middleware.NoStore)
			if deps.AuthLimiter != nil {
				r.Use(deps.AuthLimiter.Middleware)
			}
			deps.Auth.RegisterRoutes(r)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAuth(deps.TokenVerifier, deps.Logger))

			r.Route("/users", deps.User.RegisterRoutes)
			r.Route("/recommend", deps.Recommendation.RegisterRoutes)
			r.Route("/farms", func(r chi.Router) {
				deps.Farm.RegisterRoutes(r, deps.Weather)
			})
		})
	})

	return r
}
