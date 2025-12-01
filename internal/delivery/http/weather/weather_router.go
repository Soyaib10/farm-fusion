package weather

import (
	"github.com/go-chi/chi/v5"
)

func RegisterRoutes(r chi.Router, h *Handler) {
	r.Post("/", h.Create)
	r.Get("/", h.List)
	r.Get("/{weatherID}", h.GetByID)
	r.Put("/{weatherID}", h.Update)
	r.Delete("/{weatherID}", h.Delete)
}
