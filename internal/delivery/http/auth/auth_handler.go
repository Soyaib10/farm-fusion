package auth

import (
	"net/http"

	"github.com/Soyaib10/farm-fusion/internal/app"
	"github.com/Soyaib10/farm-fusion/internal/usecase/auth"
)

type Handler struct {
	app     *app.Application
	usecase auth.UseCase
}

func NewHandler(app *app.Application, usecase auth.UseCase) *Handler {
	return &Handler{
		app:     app,
		usecase: usecase,
	}
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var input RegisterRequestJSON
	if err := h.app.ReadJSON(w, r, &input); err != nil {
		h.app.BadRequestResponse(w, r, err)
		return
	}

	cmd := auth.RegisterCommand{
		Name:     input.Name,
		Email:    input.Email,
		Password: input.Password,
	}

	resp, err := h.usecase.Register(r.Context(), cmd)
	if err != nil {
		h.app.ServerErrorResponse(w, r, err)
		return
	}

	if err := h.app.WriteJSON(w, http.StatusCreated, app.Envelope{"auth": resp}, nil); err != nil {
		h.app.ServerErrorResponse(w, r, err)
	}
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var input LoginRequestJSON
	if err := h.app.ReadJSON(w, r, &input); err != nil {
		h.app.BadRequestResponse(w, r, err)
		return
	}

	cmd := auth.LoginCommand{
		Email:    input.Email,
		Password: input.Password,
	}

	resp, err := h.usecase.Login(r.Context(), cmd)
	if err != nil {
		// TODO: Distinguish between invalid creds and server error
		h.app.InvalidCredentialsResponse(w, r)
		return
	}

	response := AuthResponseJSON{
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
		User:         toUserJSON(resp.User),
	}

	if err := h.app.WriteJSON(w, http.StatusOK, app.Envelope{"auth": response}, nil); err != nil {
		h.app.ServerErrorResponse(w, r, err)
	}
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	var input RefreshRequestJSON
	if err := h.app.ReadJSON(w, r, &input); err != nil {
		h.app.BadRequestResponse(w, r, err)
		return
	}

	resp, err := h.usecase.Refresh(r.Context(), input.RefreshToken)
	if err != nil {
		h.app.InvalidAuthenticationTokenResponse(w, r)
		return
	}

	response := AuthResponseJSON{
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
		User:         toUserJSON(resp.User),
	}

	if err := h.app.WriteJSON(w, http.StatusOK, app.Envelope{"auth": response}, nil); err != nil {
		h.app.ServerErrorResponse(w, r, err)
	}
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	var input RefreshRequestJSON
	if err := h.app.ReadJSON(w, r, &input); err != nil {
		h.app.BadRequestResponse(w, r, err)
		return
	}

	err := h.usecase.Logout(r.Context(), input.RefreshToken)
	if err != nil {
		h.app.ServerErrorResponse(w, r, err)
		return
	}

	if err := h.app.WriteJSON(w, http.StatusOK, app.Envelope{"message": "logged out successfully"}, nil); err != nil {
		h.app.ServerErrorResponse(w, r, err)
	}
}
