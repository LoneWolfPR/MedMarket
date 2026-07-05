package http

import (
	"net/http"

	"github.com/LoneWolfPR/MedMarket/backend/internal/adapters/inbound/http/openapi"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/outbound"
)

// NewAPI assembles the auth handler, JWT middleware, and error-rendering policy
// into a routable ServerInterface. It is the http package's single exported seam:
// the middleware, context key, and error helpers stay unexported here.
func NewAPI(h *AuthHandler, ti outbound.TokenIssuer) openapi.ServerInterface {
	protected := map[string]struct{}{"GetProfile": {}}
	authMW := newAuthMiddleware(ti, protected)

	opts := openapi.StrictHTTPServerOptions{
		RequestErrorHandlerFunc: func(
			w http.ResponseWriter,
			_ *http.Request,
			err error,
		) {
			writeJSONError(w, http.StatusBadRequest, err.Error())
		},
		ResponseErrorHandlerFunc: func(
			w http.ResponseWriter,
			_ *http.Request,
			err error,
		) {
			h.logger.Error("unexpected handler error", "error", err)
			writeJSONError(w, http.StatusInternalServerError, msgInternalServerError)
		},
	}

	return openapi.NewStrictHandlerWithOptions(h, []openapi.StrictMiddlewareFunc{authMW}, opts)

}
