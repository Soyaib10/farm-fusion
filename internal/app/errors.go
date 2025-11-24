package app

import (
	"fmt"
	"net/http"
)

func (app *Application) LogError(r *http.Request, err error) {
	app.Logger.PrintError(err, map[string]string{
		"request_method": r.Method,
		"request_url":    r.URL.String(),
	})
}

func (app *Application) ErrorResponse(w http.ResponseWriter, r *http.Request, status int, message interface{}) {
	env := Envelope{"error": message}
	err := app.WriteJSON(w, status, env, nil)
	if err != nil {
		app.LogError(r, err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (app *Application) ServerErrorResponse(w http.ResponseWriter, r *http.Request, err error) {
	app.LogError(r, err)
	message := "the server encountered a problem and could not process your request"
	app.ErrorResponse(w, r, http.StatusInternalServerError, message)
}

func (app *Application) NotFoundResponse(w http.ResponseWriter, r *http.Request) {
	message := "the requested resource could not be found"
	app.ErrorResponse(w, r, http.StatusNotFound, message)
}

func (app *Application) MethodNotAllowedResponse(w http.ResponseWriter, r *http.Request) {
	message := fmt.Sprintf("the %s method is not supported for this resource", r.Method)
	app.ErrorResponse(w, r, http.StatusMethodNotAllowed, message)
}

func (app *Application) BadRequestResponse(w http.ResponseWriter, r *http.Request, err error) {
	app.ErrorResponse(w, r, http.StatusBadRequest, err.Error())
}

func (app *Application) FailedValidationResponse(w http.ResponseWriter, r *http.Request, errors map[string]string) {
	app.ErrorResponse(w, r, http.StatusUnprocessableEntity, errors)
}

func (app *Application) EditConflictResponse(w http.ResponseWriter, r *http.Request) {
	message := "unable to update the record due to an edit conflict, please try again"
	app.ErrorResponse(w, r, http.StatusConflict, message)
}

func (app *Application) RateLimitExceededResponse(w http.ResponseWriter, r *http.Request) {
	message := "rate limit exceeded"
	app.ErrorResponse(w, r, http.StatusTooManyRequests, message)
}
