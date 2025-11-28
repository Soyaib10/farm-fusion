package validator

import (
	"fmt"
	"regexp"
)

var (
	EmailRX = regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+\\/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")
)

type ValidationError map[string]string

func (ve ValidationError) Error() string {
	return fmt.Sprintf("validation failed: %d errors", len(ve))
}

type Validator struct {
	Errors ValidationError
}

func New() *Validator {
	return &Validator{Errors: make(ValidationError)}
}

func (v *Validator) Valid() bool {
	return len(v.Errors) == 0
}

func (v *Validator) AddError(key, message string) {
	if _, exists := v.Errors[key]; !exists {
		v.Errors[key] = message
	}
}

func (v *Validator) Check(ok bool, key, message string) {
	if !ok {
		v.AddError(key, message)
	}
}

func In(value string, list ...string) bool {
	for _, v := range list {
		if value == v {
			return true
		}
	}
	return false
}

func Matches(value string, rx *regexp.Regexp) bool {
	return rx.MatchString(value)
}

func Unique[T comparable](values []T) bool {
	seen := make(map[T]bool)
	for _, v := range values {
		if seen[v] {
			return false
		}
		seen[v] = true
	}
	return true
}
