package response

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/Soyaib10/farm-fusion/internal/platform/logger"
)

// Error writes a structured JSON error payload.
func Error(w http.ResponseWriter, r *http.Request, log *logger.Logger, status int, message any) {
	if err := WriteJSON(w, status, Envelope{"error": message}, nil); err != nil {
		logRequestError(r, log, err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// BadRequest writes a 400 Bad Request JSON error.
func BadRequest(w http.ResponseWriter, r *http.Request, log *logger.Logger, err error) {
	Error(w, r, log, http.StatusBadRequest, err.Error())
}

// UnauthorizedBearer writes a 401 Unauthorized response with WWW-Authenticate: Bearer.
func UnauthorizedBearer(w http.ResponseWriter, r *http.Request, log *logger.Logger, message string) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	Error(w, r, log, http.StatusUnauthorized, message)
}

// Forbidden writes a 403 Forbidden response.
func Forbidden(w http.ResponseWriter, r *http.Request, log *logger.Logger) {
	Error(w, r, log, http.StatusForbidden, "you do not have permission to access this resource")
}

// NotFound writes a 404 Not Found response.
func NotFound(w http.ResponseWriter, r *http.Request, log *logger.Logger) {
	Error(w, r, log, http.StatusNotFound, "the requested resource could not be found")
}

// MethodNotAllowed writes a 405 Method Not Allowed response.
func MethodNotAllowed(w http.ResponseWriter, r *http.Request, log *logger.Logger) {
	msg := fmt.Sprintf("the %s method is not supported for this resource", r.Method)
	Error(w, r, log, http.StatusMethodNotAllowed, msg)
}

// RateLimitExceeded writes a 429 Too Many Requests response.
func RateLimitExceeded(w http.ResponseWriter, r *http.Request, log *logger.Logger) {
	w.Header().Set("Retry-After", "60")
	Error(w, r, log, http.StatusTooManyRequests, "rate limit exceeded, please try again later")
}

// ServerError logs the underlying error server-side and writes a generic 500 response.
func ServerError(w http.ResponseWriter, r *http.Request, log *logger.Logger, err error) {
	logRequestError(r, log, err)
	Error(w, r, log, http.StatusInternalServerError, "the server encountered a problem and could not process your request")
}

// HandleDomainError maps domain sentinel and validation errors to appropriate HTTP status codes.
func HandleDomainError(w http.ResponseWriter, r *http.Request, log *logger.Logger, err error) {
	var valErr *domain.ValidationError
	switch {
	case errors.As(err, &valErr):
		Error(w, r, log, http.StatusUnprocessableEntity, valErr.Fields)
	case errors.Is(err, domain.ErrInvalidCredentials):
		Error(w, r, log, http.StatusUnauthorized, "invalid authentication credentials")
	case errors.Is(err, domain.ErrUnauthorized),
		errors.Is(err, domain.ErrTokenExpired),
		errors.Is(err, domain.ErrTokenRevoked):
		UnauthorizedBearer(w, r, log, "invalid or expired authentication token")
	case errors.Is(err, domain.ErrForbidden):
		Forbidden(w, r, log)
	case errors.Is(err, domain.ErrNotFound):
		NotFound(w, r, log)
	case errors.Is(err, domain.ErrAlreadyExists):
		Error(w, r, log, http.StatusConflict, "a resource with the provided details already exists")
	case errors.Is(err, domain.ErrUpstreamUnavailable):
		logRequestError(r, log, err)
		Error(w, r, log, http.StatusBadGateway, "recommendation engine is temporarily unavailable")
	default:
		ServerError(w, r, log, err)
	}
}

func logRequestError(r *http.Request, log *logger.Logger, err error) {
	if log == nil || err == nil {
		return
	}
	log.Error(err, map[string]string{
		"request_method": r.Method,
		"request_path":   r.URL.Path,
	})
}
