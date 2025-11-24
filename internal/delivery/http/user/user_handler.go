package user

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/Soyaib10/farm-fusion/internal/app"
	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/Soyaib10/farm-fusion/internal/usecase/user"
	"github.com/Soyaib10/farm-fusion/internal/validator"
)

type Handler struct {
	app     *app.Application
	usecase user.UseCase
}

func NewHandler(app *app.Application, usecase user.UseCase) *Handler {
	return &Handler{
		app:     app,
		usecase: usecase,
	}
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var input UpsertUserJSON
	if err := h.app.ReadJSON(w, r, &input); err != nil {
		h.app.BadRequestResponse(w, r, err)
		return
	}

	cmd := user.UpsertUserCommand{
		Name:  input.Name,
		Email: input.Email,
	}

	created, err := h.usecase.Create(r.Context(), cmd)
	if err != nil {
		if validationErrors, ok := err.(validator.ValidationError); ok {
			h.app.FailedValidationResponse(w, r, validationErrors)
			return
		}
		h.app.ServerErrorResponse(w, r, err)
		return
	}

	headers := make(http.Header)
	headers.Set("Location", fmt.Sprintf("/v1/users/%s", created.ID))

	err = h.app.WriteJSON(w, http.StatusCreated, app.Envelope{"user": created}, headers)
	if err != nil {
		h.app.ServerErrorResponse(w, r, err)
	}
}

func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := h.app.ReadIDParam(r)
	if err != nil {
		h.app.NotFoundResponse(w, r)
		return
	}

	user, err := h.usecase.GetByID(r.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrNotFound):
			h.app.NotFoundResponse(w, r)
		default:
			h.app.ServerErrorResponse(w, r, err)
		}
		return
	}

	err = h.app.WriteJSON(w, http.StatusOK, app.Envelope{"user": user}, nil)
	if err != nil {
		h.app.ServerErrorResponse(w, r, err)
	}
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := h.app.ReadIDParam(r)
	if err != nil {
		h.app.NotFoundResponse(w, r)
		return
	}

	var input UpsertUserJSON
	if err := h.app.ReadJSON(w, r, &input); err != nil {
		h.app.BadRequestResponse(w, r, err)
		return
	}

	cmd := user.UpsertUserCommand{
		Name:  input.Name,
		Email: input.Email,
	}

	updated, err := h.usecase.Update(r.Context(), id, cmd)
	if err != nil {
		if validationErrors, ok := err.(validator.ValidationError); ok {
			h.app.FailedValidationResponse(w, r, validationErrors)
			return
		}
		switch {
		case errors.Is(err, domain.ErrNotFound):
			h.app.NotFoundResponse(w, r)
		default:
			h.app.ServerErrorResponse(w, r, err)
		}
		return
	}

	err = h.app.WriteJSON(w, http.StatusOK, app.Envelope{"user": updated}, nil)
	if err != nil {
		h.app.ServerErrorResponse(w, r, err)
	}
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := h.app.ReadIDParam(r)
	if err != nil {
		h.app.NotFoundResponse(w, r)
		return
	}

	err = h.usecase.Delete(r.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrNotFound):
			h.app.NotFoundResponse(w, r)
		default:
			h.app.ServerErrorResponse(w, r, err)
		}
		return
	}

	err = h.app.WriteJSON(w, http.StatusOK, app.Envelope{"message": "user successfully deleted"}, nil)
	if err != nil {
		h.app.ServerErrorResponse(w, r, err)
	}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	users, err := h.usecase.List(r.Context())
	if err != nil {
		h.app.ServerErrorResponse(w, r, err)
		return
	}

	err = h.app.WriteJSON(w, http.StatusOK, app.Envelope{"users": users}, nil)
	if err != nil {
		h.app.ServerErrorResponse(w, r, err)
	}
}
