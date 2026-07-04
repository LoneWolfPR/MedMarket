package http

import (
	"context"

	"github.com/google/uuid"
)

type ctxKey struct{}

var userIDKey ctxKey

func setUserIDKeyValue(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, userIDKey, id)
}

func getUserIDKeyValue(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(userIDKey).(uuid.UUID)
	return id, ok
}
