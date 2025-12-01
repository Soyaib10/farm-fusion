package weather

import (
	"errors"
	"net/http"

	"github.com/Soyaib10/farm-fusion/internal/app"
	"github.com/Soyaib10/farm-fusion/internal/delivery/http/middleware"
	"github.com/Soyaib10/farm-fusion/internal/usecase/weather"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	app     *app.Application
	usecase weather.UseCase
}

func NewHandler(app *app.Application, usecase weather.UseCase) *Handler {
	return &Handler{
		app:     app,
		usecase: usecase,
	}
}

func getFarmIDFromURL(r *http.Request) (uuid.UUID, error) {
	farmIDStr := chi.URLParam(r, "farmID")
	farmID, err := uuid.Parse(farmIDStr)
	if err != nil {
		return uuid.Nil, errors.New("invalid farm ID format")
	}
	return farmID, nil
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	farmID, err := getFarmIDFromURL(r)
	if err != nil {
		h.app.BadRequestResponse(w, r, err)
		return
	}

	userID, err := middleware.GetUserIDFromContext(r)
	if err != nil {
		h.app.ServerErrorResponse(w, r, err)
		return
	}

	var input CreateWeatherJSON
	if err := h.app.ReadJSON(w, r, &input); err != nil {
		h.app.BadRequestResponse(w, r, err)
		return
	}

	cmd := weather.CreateWeatherCommand{
		UserID:   userID,
		FarmID:   farmID,
		Metric:   input.Metric,
		Operator: input.Operator,
		Value:    input.Value,
	}

	newWeather, err := h.usecase.Create(r.Context(), cmd)
	if err != nil {
		h.app.ServerErrorResponse(w, r, err)
		return
	}

	response := WeatherResponse{
		ID:        newWeather.ID,
		FarmID:    newWeather.FarmID,
		Metric:    newWeather.Metric,
		Operator:  newWeather.Operator,
		Value:     newWeather.Value,
		IsEnabled: newWeather.IsEnabled,
	}

	if err := h.app.WriteJSON(w, http.StatusCreated, app.Envelope{"weather": response}, nil); err != nil {
		h.app.ServerErrorResponse(w, r, err)
	}
}

func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	farmID, err := getFarmIDFromURL(r)
	if err != nil {
		h.app.BadRequestResponse(w, r, err)
		return
	}

	weatherIDStr := chi.URLParam(r, "weatherID")
	weatherID, err := uuid.Parse(weatherIDStr)
	if err != nil {
		h.app.BadRequestResponse(w, r, errors.New("invalid weather ID format"))
		return
	}

	userID, err := middleware.GetUserIDFromContext(r)
	if err != nil {
		h.app.ServerErrorResponse(w, r, err)
		return
	}

	weather, err := h.usecase.GetByID(r.Context(), userID, farmID, weatherID)
	if err != nil {
		h.app.ServerErrorResponse(w, r, err)
		return
	}

	response := WeatherResponse{
		ID:        weather.ID,
		FarmID:    weather.FarmID,
		Metric:    weather.Metric,
		Operator:  weather.Operator,
		Value:     weather.Value,
		IsEnabled: weather.IsEnabled,
	}

	if err := h.app.WriteJSON(w, http.StatusOK, app.Envelope{"weather": response}, nil); err != nil {
		h.app.ServerErrorResponse(w, r, err)
	}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	farmID, err := getFarmIDFromURL(r)
	if err != nil {
		h.app.BadRequestResponse(w, r, err)
		return
	}

	userID, err := middleware.GetUserIDFromContext(r)
	if err != nil {
		h.app.ServerErrorResponse(w, r, err)
		return
	}

	weathers, err := h.usecase.ListByFarm(r.Context(), userID, farmID)
	if err != nil {
		h.app.ServerErrorResponse(w, r, err)
		return
	}

	weatherResponses := make([]WeatherResponse, len(weathers))
	for i, weather := range weathers {
		weatherResponses[i] = WeatherResponse{
			ID:        weather.ID,
			FarmID:    weather.FarmID,
			Metric:    weather.Metric,
			Operator:  weather.Operator,
			Value:     weather.Value,
			IsEnabled: weather.IsEnabled,
		}
	}

	if err := h.app.WriteJSON(w, http.StatusOK, app.Envelope{"weathers": weatherResponses}, nil); err != nil {
		h.app.ServerErrorResponse(w, r, err)
	}
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	farmID, err := getFarmIDFromURL(r)
	if err != nil {
		h.app.BadRequestResponse(w, r, err)
		return
	}

	weatherIDStr := chi.URLParam(r, "weatherID")
	weatherID, err := uuid.Parse(weatherIDStr)
	if err != nil {
		h.app.BadRequestResponse(w, r, errors.New("invalid weather ID format"))
		return
	}

	userID, err := middleware.GetUserIDFromContext(r)
	if err != nil {
		h.app.ServerErrorResponse(w, r, err)
		return
	}

	var input UpdateWeatherJSON
	if err := h.app.ReadJSON(w, r, &input); err != nil {
		h.app.BadRequestResponse(w, r, err)
		return
	}

	cmd := weather.UpdateWeatherCommand{
		UserID:    userID,
		FarmID:    farmID,
		WeatherID: weatherID,
		Metric:    input.Metric,
		Operator:  input.Operator,
		Value:     input.Value,
		IsEnabled: input.IsEnabled,
	}

	updatedWeather, err := h.usecase.Update(r.Context(), cmd)
	if err != nil {
		h.app.ServerErrorResponse(w, r, err)
		return
	}

	response := WeatherResponse{
		ID:        updatedWeather.ID,
		FarmID:    updatedWeather.FarmID,
		Metric:    updatedWeather.Metric,
		Operator:  updatedWeather.Operator,
		Value:     updatedWeather.Value,
		IsEnabled: updatedWeather.IsEnabled,
	}

	if err := h.app.WriteJSON(w, http.StatusOK, app.Envelope{"weather": response}, nil); err != nil {
		h.app.ServerErrorResponse(w, r, err)
	}
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	farmID, err := getFarmIDFromURL(r)
	if err != nil {
		h.app.BadRequestResponse(w, r, err)
		return
	}

	weatherIDStr := chi.URLParam(r, "weatherID")
	weatherID, err := uuid.Parse(weatherIDStr)
	if err != nil {
		h.app.BadRequestResponse(w, r, errors.New("invalid weather ID format"))
		return
	}

	userID, err := middleware.GetUserIDFromContext(r)
	if err != nil {
		h.app.ServerErrorResponse(w, r, err)
		return
	}

	if err := h.usecase.Delete(r.Context(), userID, farmID, weatherID); err != nil {
		h.app.ServerErrorResponse(w, r, err)
		return
	}

	if err := h.app.WriteJSON(w, http.StatusOK, app.Envelope{"message": "weather deleted successfully"}, nil); err != nil {
		h.app.ServerErrorResponse(w, r, err)
	}
}
