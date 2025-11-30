package http

import (
	"github.com/Soyaib10/farm-fusion/internal/app"
	"github.com/Soyaib10/farm-fusion/internal/config"
	"github.com/Soyaib10/farm-fusion/internal/delivery/http/auth"
	"github.com/Soyaib10/farm-fusion/internal/delivery/http/farm"
	"github.com/Soyaib10/farm-fusion/internal/delivery/http/middleware"
	"github.com/Soyaib10/farm-fusion/internal/delivery/http/recommendation"
	"github.com/Soyaib10/farm-fusion/internal/delivery/http/user"
	"github.com/go-chi/chi/v5"
)

type Handlers struct {
	Auth           *auth.Handler
	User           *user.Handler
	Recommendation *recommendation.Handler
	Farm           *farm.Handler
}

func NewHandlers(h *Handlers, cfg *config.Config, app *app.Application) *chi.Mux {
	r := chi.NewRouter()

	jwtMiddleware := middleware.NewJWTMiddleware(app, cfg)

	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			auth.RegisterRoutes(r, h.Auth)
		})
		r.Group(func(r chi.Router) {
			r.Use(jwtMiddleware.RequireAuthenticatedUser)
			r.Route("/users", func(r chi.Router) {
				user.RegisterRoutes(r, h.User)
			})
			r.Route("/recommend", func(r chi.Router) {
				recommendation.RegisterRoutes(r, h.Recommendation)
			})
			r.Route("/farms", func(r chi.Router) {
				farm.RegisterRoutes(r, h.Farm)
			})
		})
	})

	return r
}
