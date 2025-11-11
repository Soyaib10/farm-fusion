package user

import "github.com/go-chi/chi/v5"

func RegisterRoutes(r chi.Router, h *Handler) {
    r.Post("/", h.Create)
    r.Get("/{id}", h.GetByID)
}


