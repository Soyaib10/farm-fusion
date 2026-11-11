package response

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const maxBodyBytes = 1 << 20 // 1 MiB

// Envelope wraps JSON response payloads under named top-level keys.
type Envelope map[string]any

// ReadJSON decodes a single JSON object from r.Body into dst, enforcing a 1 MiB size limit
// and rejecting unknown fields.
func ReadJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		var (
			syntaxErr        *json.SyntaxError
			unmarshalTypeErr *json.UnmarshalTypeError
			invalidUnmarshal *json.InvalidUnmarshalError
			maxBytesErr      *http.MaxBytesError
		)

		switch {
		case errors.As(err, &syntaxErr):
			return fmt.Errorf("body contains badly-formed JSON (at character %d)", syntaxErr.Offset)
		case errors.Is(err, io.ErrUnexpectedEOF):
			return errors.New("body contains badly-formed JSON")
		case errors.As(err, &unmarshalTypeErr):
			if unmarshalTypeErr.Field != "" {
				return fmt.Errorf("body contains incorrect JSON type for field %q", unmarshalTypeErr.Field)
			}
			return fmt.Errorf("body contains incorrect JSON type (at character %d)", unmarshalTypeErr.Offset)
		case errors.Is(err, io.EOF):
			return errors.New("body must not be empty")
		case strings.HasPrefix(err.Error(), "json: unknown field "):
			field := strings.TrimPrefix(err.Error(), "json: unknown field ")
			return fmt.Errorf("body contains unknown key %s", field)
		case errors.As(err, &maxBytesErr):
			return fmt.Errorf("body must not be larger than %d bytes", maxBytesErr.Limit)
		case errors.As(err, &invalidUnmarshal):
			panic(err)
		default:
			return errors.New("unable to parse JSON request body")
		}
	}

	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("body must only contain a single JSON value")
	}

	return nil
}

// WriteJSON serializes data as JSON and writes it with status and optional headers.
func WriteJSON(w http.ResponseWriter, status int, data Envelope, headers http.Header) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}
	payload = append(payload, '\n')

	for k, vals := range headers {
		w.Header()[k] = vals
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, err = w.Write(payload)
	return err
}

// ReadUUIDParam extracts and parses a named UUID URL parameter from Chi route context.
func ReadUUIDParam(r *http.Request, param string) (uuid.UUID, error) {
	raw := chi.URLParam(r, param)
	id, err := uuid.Parse(raw)
	if err != nil || id == uuid.Nil {
		return uuid.Nil, fmt.Errorf("invalid %s parameter", param)
	}
	return id, nil
}
