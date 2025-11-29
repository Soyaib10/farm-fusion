package recommendation

import (
	"net/http"

	"github.com/Soyaib10/farm-fusion/internal/app"
	"github.com/Soyaib10/farm-fusion/internal/usecase/recommendation"
)

type Handler struct {
	app     *app.Application
	usecase recommendation.UseCase
}

func NewHandler(app *app.Application, usecase recommendation.UseCase) *Handler {
	return &Handler{
		app:     app,
		usecase: usecase,
	}
}

func (h *Handler) CropRecommendation(w http.ResponseWriter, r *http.Request) {
	var input CropRequestJSON
	if err := h.app.ReadJSON(w, r, &input); err != nil {
		h.app.BadRequestResponse(w, r, err)
		return
	}

	cmd := recommendation.CropCommand{
		N:           input.N,
		P:           input.P,
		K:           input.K,
		Temperature: input.Temperature,
		Humidity:    input.Humidity,
		PH:          input.PH,
		Rainfall:    input.Rainfall,
	}

	result, err := h.usecase.CropRecommendation(r.Context(), cmd)
	if err != nil {
		h.app.ServerErrorResponse(w, r, err)
		return
	}

	if err := h.app.WriteJSON(w, http.StatusOK, app.Envelope{"recommendation": result}, nil); err != nil {
		h.app.ServerErrorResponse(w, r, err)
	}
}

func (h *Handler) FertilizerRecommendation(w http.ResponseWriter, r *http.Request) {
	var input FertilizerRequestJSON
	if err := h.app.ReadJSON(w, r, &input); err != nil {
		h.app.BadRequestResponse(w, r, err)
		return
	}

	cmd := recommendation.FertilizerCommand{
		Temperature:  input.Temperature,
		Humidity:     input.Humidity,
		SoilMoisture: input.SoilMoisture,
		SoilType:     input.SoilType,
		CropType:     input.CropType,
		Nitrogen:     input.Nitrogen,
		Potassium:    input.Potassium,
		Phosphorous:  input.Phosphorous,
	}

	result, err := h.usecase.FertilizerRecommendation(r.Context(), cmd)
	if err != nil {
		h.app.ServerErrorResponse(w, r, err)
		return
	}

	if err := h.app.WriteJSON(w, http.StatusOK, app.Envelope{"recommendation": result}, nil); err != nil {
		h.app.ServerErrorResponse(w, r, err)
	}
}
