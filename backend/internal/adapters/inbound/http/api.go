package http

import (
	"log/slog"
	"net/http"

	"github.com/LoneWolfPR/MedMarket/backend/internal/adapters/inbound/http/openapi"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/outbound"
)

type server struct {
	*AuthHandler
	*PrescriptionHandler
}

var _ openapi.StrictServerInterface = (*server)(nil)

// NewAPIParams contains the parameters for starting up the api
type NewAPIParams struct {
	Auth         *AuthHandler
	Prescription *PrescriptionHandler
	Logger       *slog.Logger
	TokenIssuer  outbound.TokenIssuer
}

// NewAPI assembles the auth handler, JWT middleware, and error-rendering policy
// into a routable ServerInterface. It is the http package's single exported seam:
// the middleware, context key, and error helpers stay unexported here.
func NewAPI(p NewAPIParams) openapi.ServerInterface {
	s := &server{
		AuthHandler:         p.Auth,
		PrescriptionHandler: p.Prescription,
	}
	protected := map[string]struct{}{
		"GetProfile":         {},
		"UploadPrescription": {},
		"ListPrescriptions":  {},
	}
	authMW := newAuthMiddleware(p.TokenIssuer, protected)

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
			p.Logger.Error("unexpected handler error", "error", err)
			writeJSONError(w, http.StatusInternalServerError, msgInternalServerError)
		},
	}

	return openapi.NewStrictHandlerWithOptions(s, []openapi.StrictMiddlewareFunc{authMW}, opts)

}
