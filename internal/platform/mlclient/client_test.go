package mlclient

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/Soyaib10/farm-fusion/internal/platform/logger"
)

func newTestClient(url string) *Client {
	return NewClient(url, "0123456789abcdef0123456789abcdef", http.DefaultClient, logger.New(io.Discard, logger.LevelOff))
}

func TestPostJSONSendsKeyAndMapsStatuses(t *testing.T) {
	status := http.StatusOK
	var gotKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get(internalKeyHeader)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"crop":"rice","confidence":0.9}`))
	}))
	defer srv.Close()
	c := newTestClient(srv.URL)

	if _, err := c.PredictCrop(context.Background(), domain.CropQuery{}); err != nil {
		t.Fatalf("PredictCrop() 200: %v", err)
	}
	if gotKey == "" {
		t.Fatal("expected X-Internal-API-Key on the request")
	}

	status = http.StatusUnprocessableEntity
	var valErr *domain.ValidationError
	if _, err := c.PredictCrop(context.Background(), domain.CropQuery{}); !errors.As(err, &valErr) {
		t.Fatalf("422 got %v, want ValidationError", err)
	}

	status = http.StatusInternalServerError
	if _, err := c.PredictCrop(context.Background(), domain.CropQuery{}); !errors.Is(err, domain.ErrUpstreamUnavailable) {
		t.Fatalf("500 got %v, want ErrUpstreamUnavailable", err)
	}
}

func TestPostJSONPreservesCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	c := newTestClient(srv.URL)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := c.PredictCrop(ctx, domain.CropQuery{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want an error wrapping context.Canceled", err)
	}
	if !errors.Is(err, domain.ErrUpstreamUnavailable) {
		t.Fatalf("got %v, want an error wrapping ErrUpstreamUnavailable", err)
	}
}
