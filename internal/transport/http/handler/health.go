package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/Soyaib10/farm-fusion/internal/platform/logger"
	"github.com/Soyaib10/farm-fusion/internal/transport/http/response"
)

// Pinger checks readiness of a backing dependency such as PostgreSQL.
type Pinger interface {
	Ping(ctx context.Context) error
}

// HealthHandler serves liveness and readiness probes.
type HealthHandler struct {
	db  Pinger
	log *logger.Logger
}

// NewHealthHandler constructs a HealthHandler.
func NewHealthHandler(db Pinger, log *logger.Logger) *HealthHandler {
	return &HealthHandler{db: db, log: log}
}

// Live responds 200 OK if the HTTP server process is responsive.
func (h *HealthHandler) Live(w http.ResponseWriter, r *http.Request) {
	if err := response.WriteJSON(w, http.StatusOK, response.Envelope{"status": "ok"}, nil); err != nil {
		response.ServerError(w, r, h.log, err)
	}
}

// Ready verifies database connectivity and responds 200 OK or 503 Service Unavailable.
func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	if h.db != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := h.db.Ping(ctx); err != nil {
			response.Error(w, r, h.log, http.StatusServiceUnavailable, "database unavailable")
			return
		}
	}
	if err := response.WriteJSON(w, http.StatusOK, response.Envelope{"status": "ready"}, nil); err != nil {
		response.ServerError(w, r, h.log, err)
	}
}
