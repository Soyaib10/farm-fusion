package farm

import (
	"github.com/Soyaib10/farm-fusion/internal/delivery/http/weather"
	"github.com/go-chi/chi/v5"
)

func RegisterRoutes(r chi.Router, farmHandler *Handler, weatherHandler *weather.Handler) {
	r.Post("/", farmHandler.Create)
	r.Get("/", farmHandler.ListByUserID)

	r.Route("/{farmID}", func(r chi.Router) {
		r.Get("/", farmHandler.GetByID)
		r.Put("/", farmHandler.Update)
		r.Delete("/", farmHandler.Delete)

		r.Route("/weather-alerts", func(r chi.Router) {
			weather.RegisterRoutes(r, weatherHandler)
		})
	})
}
