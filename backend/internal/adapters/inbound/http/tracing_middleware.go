package http

import (
	"context"
	"net/http"

	"github.com/LoneWolfPR/MedMarket/backend/internal/adapters/inbound/http/openapi"

	"go.opentelemetry.io/otel/trace"
)

func newTracingMiddleware() openapi.StrictMiddlewareFunc {
	return func(next openapi.StrictHandlerFunc, operationID string) openapi.StrictHandlerFunc {
		return func(
			ctx context.Context,
			w http.ResponseWriter,
			r *http.Request,
			request any,
		) (any, error) {
			trace.SpanFromContext(ctx).SetName(operationID)
			return next(ctx, w, r, request)
		}
	}
}
