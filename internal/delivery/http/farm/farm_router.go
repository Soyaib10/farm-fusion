package farm

import (
	"github.com/go-chi/chi/v5"
)

func RegisterRoutes(r chi.Router, h *Handler) {
	r.Post("/", h.Create)
	r.Get("/", h.List)
	r.Get("/{farmID}", h.Get)
	r.Put("/{farmID}", h.Update)
	r.Delete("/{farmID}", h.Delete)
}

