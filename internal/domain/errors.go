package domain

import (
	"errors"
	"fmt"
)

// Sentinel domain errors used across services, repositories, and transports.
var (
	ErrNotFound            = errors.New("resource not found")
	ErrAlreadyExists       = errors.New("resource already exists")
	ErrForbidden           = errors.New("forbidden")
	ErrUnauthorized        = errors.New("unauthorized")
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrTokenExpired        = errors.New("token expired")
	ErrTokenRevoked        = errors.New("token revoked")
	ErrUpstreamUnavailable = errors.New("upstream service unavailable")
)

// ValidationError maps field names to human-readable validation failure reasons.
type ValidationError struct {
	Fields map[string]string
}

// NewValidationError wraps field errors into a domain ValidationError.
func NewValidationError(fields map[string]string) *ValidationError {
	return &ValidationError{Fields: fields}
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation failed (%d field error(s))", len(e.Fields))
}
