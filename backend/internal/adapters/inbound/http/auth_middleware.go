package http

import (
	"context"
	"net/http"
	"strings"

	"github.com/LoneWolfPR/MedMarket/backend/internal/adapters/inbound/http/openapi"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/outbound"
)

const (
	headerNameAuth   = "Authorization"
	authHeaderScheme = "Bearer"
)

func newAuthMiddleware(ti outbound.TokenIssuer, protected map[string]struct{}) openapi.StrictMiddlewareFunc {
	return func(next openapi.StrictHandlerFunc, operationID string) openapi.StrictHandlerFunc {
		if _, ok := protected[operationID]; ok {
			return func(
				ctx context.Context,
				w http.ResponseWriter,
				r *http.Request,
				request any,
			) (any, error) {
				authHeader := r.Header.Get(headerNameAuth)
				if authHeader == "" {
					writeJSONError(w, http.StatusUnauthorized, msgUnauthorized)
					return nil, nil
				}
				scheme, token, found := strings.Cut(authHeader, " ")
				if !found {
					writeJSONError(w, http.StatusUnauthorized, msgUnauthorized)
					return nil, nil
				}
				if !strings.EqualFold(scheme, authHeaderScheme) {
					writeJSONError(w, http.StatusUnauthorized, msgUnauthorized)
					return nil, nil
				}

				id, err := ti.Verify(token)
				if err != nil {
					writeJSONError(w, http.StatusUnauthorized, msgUnauthorized)
					return nil, nil
				}

				return next(setUserIDKeyValue(ctx, id), w, r, request)
			}
		}
		return next
	}
}
