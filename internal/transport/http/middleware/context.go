package middleware

import (
	"context"

	"github.com/Soyaib10/farm-fusion/internal/domain"
)

type contextKey uint8

const (
	actorKey contextKey = iota + 1
	requestIDKey
)

// WithActor attaches an authenticated domain.Actor to ctx.
func WithActor(ctx context.Context, actor domain.Actor) context.Context {
	return context.WithValue(ctx, actorKey, actor)
}

// ActorFromContext extracts the authenticated domain.Actor from ctx.
func ActorFromContext(ctx context.Context) (domain.Actor, bool) {
	actor, ok := ctx.Value(actorKey).(domain.Actor)
	return actor, ok
}

// WithRequestID attaches a correlation request ID to ctx.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

// RequestIDFromContext extracts the correlation request ID from ctx.
func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}
