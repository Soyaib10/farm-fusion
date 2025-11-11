package http

import (
	"github.com/Soyaib10/farm-fusion/internal/delivery/http/user"
	"github.com/go-chi/chi/v5"
)

// If User were user (lowercase) in the Handlers struct definition, the line User: userHandler, in main.go would result in a compile-time error because user would be an unexported field and inaccessible from main.go
type Handlers struct {
	User *user.Handler
}

func NewHandlers(h *Handlers) *chi.Mux {
	r := chi.NewRouter()

	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/users", func(r chi.Router) {
			user.RegisterRoutes(r, h.User)
		})
	})

	return r
}
