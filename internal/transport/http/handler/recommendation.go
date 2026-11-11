package handler

import (
	"context"
	"net/http"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/Soyaib10/farm-fusion/internal/platform/logger"
	"github.com/Soyaib10/farm-fusion/internal/transport/http/response"
	"github.com/Soyaib10/farm-fusion/internal/validator"
	"github.com/go-chi/chi/v5"
)

// RecommendationService defines ML recommendation operations required by RecommendationHandler.
type RecommendationService interface {
	RecommendCrop(ctx context.Context, query domain.CropQuery) (*domain.CropRecommendation, error)
	RecommendFertilizer(ctx context.Context, query domain.FertilizerQuery) (*domain.FertilizerRecommendation, error)
}

// RecommendationHandler handles HTTP requests for crop and fertilizer ML predictions.
type RecommendationHandler struct {
	svc RecommendationService
	log *logger.Logger
}

// NewRecommendationHandler constructs a RecommendationHandler.
func NewRecommendationHandler(svc RecommendationService, log *logger.Logger) *RecommendationHandler {
	return &RecommendationHandler{svc: svc, log: log}
}

// RegisterRoutes mounts recommendation endpoints onto r.
func (h *RecommendationHandler) RegisterRoutes(r chi.Router) {
	r.Post("/crop", h.RecommendCrop)
	r.Post("/fertilizer", h.RecommendFertilizer)
}

type cropRequest struct {
	N           *float64 `json:"N"`
	P           *float64 `json:"P"`
	K           *float64 `json:"K"`
	Temperature *float64 `json:"temperature"`
	Humidity    *float64 `json:"humidity"`
	PH          *float64 `json:"ph"`
	Rainfall    *float64 `json:"rainfall"`
}

type fertilizerRequest struct {
	Temperature  *float64 `json:"temperature"`
	Humidity     *float64 `json:"humidity"`
	SoilMoisture *float64 `json:"soil_moisture"`
	SoilType     string   `json:"soil_type"`
	CropType     string   `json:"crop_type"`
	Nitrogen     *float64 `json:"nitrogen"`
	Potassium    *float64 `json:"potassium"`
	Phosphorous  *float64 `json:"phosphorous"`
}

// RecommendCrop handles POST /api/v1/recommend/crop.
func (h *RecommendationHandler) RecommendCrop(w http.ResponseWriter, r *http.Request) {
	var req cropRequest
	if err := response.ReadJSON(w, r, &req); err != nil {
		response.BadRequest(w, r, h.log, err)
		return
	}

	v := validator.New()
	v.Check(req.N != nil, "N", "must be provided")
	v.Check(req.P != nil, "P", "must be provided")
	v.Check(req.K != nil, "K", "must be provided")
	v.Check(req.Temperature != nil, "temperature", "must be provided")
	v.Check(req.Humidity != nil, "humidity", "must be provided")
	v.Check(req.PH != nil, "ph", "must be provided")
	v.Check(req.Rainfall != nil, "rainfall", "must be provided")
	if err := v.Err(); err != nil {
		response.HandleDomainError(w, r, h.log, err)
		return
	}

	rec, err := h.svc.RecommendCrop(r.Context(), domain.CropQuery{
		N:           *req.N,
		P:           *req.P,
		K:           *req.K,
		Temperature: *req.Temperature,
		Humidity:    *req.Humidity,
		PH:          *req.PH,
		Rainfall:    *req.Rainfall,
	})
	if err != nil {
		response.HandleDomainError(w, r, h.log, err)
		return
	}

	if err := response.WriteJSON(w, http.StatusOK, response.Envelope{"recommendation": rec}, nil); err != nil {
		response.ServerError(w, r, h.log, err)
	}
}

// RecommendFertilizer handles POST /api/v1/recommend/fertilizer.
func (h *RecommendationHandler) RecommendFertilizer(w http.ResponseWriter, r *http.Request) {
	var req fertilizerRequest
	if err := response.ReadJSON(w, r, &req); err != nil {
		response.BadRequest(w, r, h.log, err)
		return
	}

	v := validator.New()
	v.Check(req.Temperature != nil, "temperature", "must be provided")
	v.Check(req.Humidity != nil, "humidity", "must be provided")
	v.Check(req.SoilMoisture != nil, "soil_moisture", "must be provided")
	v.Check(validator.NotBlank(req.SoilType), "soil_type", "must be provided")
	v.Check(validator.NotBlank(req.CropType), "crop_type", "must be provided")
	v.Check(req.Nitrogen != nil, "nitrogen", "must be provided")
	v.Check(req.Potassium != nil, "potassium", "must be provided")
	v.Check(req.Phosphorous != nil, "phosphorous", "must be provided")
	if err := v.Err(); err != nil {
		response.HandleDomainError(w, r, h.log, err)
		return
	}

	rec, err := h.svc.RecommendFertilizer(r.Context(), domain.FertilizerQuery{
		Temperature:  *req.Temperature,
		Humidity:     *req.Humidity,
		SoilMoisture: *req.SoilMoisture,
		SoilType:     req.SoilType,
		CropType:     req.CropType,
		Nitrogen:     *req.Nitrogen,
		Potassium:    *req.Potassium,
		Phosphorous:  *req.Phosphorous,
	})
	if err != nil {
		response.HandleDomainError(w, r, h.log, err)
		return
	}

	if err := response.WriteJSON(w, http.StatusOK, response.Envelope{"recommendation": rec}, nil); err != nil {
		response.ServerError(w, r, h.log, err)
	}
}
