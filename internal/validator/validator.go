package validator

import (
	"math"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Soyaib10/farm-fusion/internal/domain"
)

// EmailRX validates standard RFC 5322 / HTML5 email addresses.
var EmailRX = regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)+$")

// Validator accumulates field-level validation errors.
type Validator struct {
	Errors map[string]string
}

// New returns an initialized Validator.
func New() *Validator {
	return &Validator{Errors: make(map[string]string)}
}

// Valid reports whether no errors were recorded.
func (v *Validator) Valid() bool {
	return len(v.Errors) == 0
}

// Err returns nil if valid, or a *domain.ValidationError containing recorded field errors.
func (v *Validator) Err() error {
	if v.Valid() {
		return nil
	}
	return domain.NewValidationError(v.Errors)
}

// AddError records the first error message for key.
func (v *Validator) AddError(key, message string) {
	if _, exists := v.Errors[key]; !exists {
		v.Errors[key] = message
	}
}

// Check records message for key when ok is false.
func (v *Validator) Check(ok bool, key, message string) {
	if !ok {
		v.AddError(key, message)
	}
}

// NotBlank reports whether s contains non-whitespace characters.
func NotBlank(s string) bool {
	return strings.TrimSpace(s) != ""
}

// RuneCountBetween reports whether s has between min and max UTF-8 runes (inclusive).
func RuneCountBetween(s string, minRunes, maxRunes int) bool {
	n := utf8.RuneCountInString(s)
	return n >= minRunes && n <= maxRunes
}

// NoControlChars reports whether s is free of ASCII/Unicode control characters (including CR, LF, NUL).
func NoControlChars(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

// Matches reports whether value matches rx.
func Matches(value string, rx *regexp.Regexp) bool {
	return rx.MatchString(value)
}

// In reports whether value is present in list.
func In[T comparable](value T, list ...T) bool {
	return slices.Contains(list, value)
}

// Finite reports whether f is neither NaN nor Inf.
func Finite(f float64) bool {
	return !math.IsNaN(f) && !math.IsInf(f, 0)
}

// Between reports whether f is finite and within [minVal, maxVal].
func Between(f, minVal, maxVal float64) bool {
	return Finite(f) && f >= minVal && f <= maxVal
}

// StrongPassword checks length (8..72 bytes for bcrypt) and requires at least one letter and one digit or symbol.
func StrongPassword(pw string) (bool, string) {
	if len(pw) < 8 {
		return false, "must be at least 8 characters long"
	}
	if len(pw) > 72 {
		return false, "must not exceed 72 bytes"
	}
	var hasLetter, hasDigitOrSymbol bool
	for _, r := range pw {
		if unicode.IsControl(r) {
			return false, "must not contain control characters"
		}
		switch {
		case unicode.IsLetter(r):
			hasLetter = true
		case unicode.IsDigit(r) || unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasDigitOrSymbol = true
		}
	}
	if !hasLetter || !hasDigitOrSymbol {
		return false, "must contain at least one letter and one number or symbol"
	}
	return true, ""
}
