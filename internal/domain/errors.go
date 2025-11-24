package domain

import "errors"

var (
	ErrNotFound      = errors.New("resource not found")
	ErrAlreadyExists = errors.New("resource already exists")
	ErrForbidden     = errors.New("forbidden")
	ErrUnauthorized  = errors.New("unauthorized")
)
