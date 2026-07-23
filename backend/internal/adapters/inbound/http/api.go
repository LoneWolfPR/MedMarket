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
	*PriceSearchHandler
	*OrderHandler
	*ShippingHandler
}

var _ openapi.StrictServerInterface = (*server)(nil)

// NewAPIParams contains the parameters for starting up the api
type NewAPIParams struct {
	Auth         *AuthHandler
	Prescription *PrescriptionHandler
	Search       *PriceSearchHandler
	Logger       *slog.Logger
	TokenIssuer  outbound.TokenIssuer
	Order        *OrderHandler
	Shipping     *ShippingHandler
}

// NewAPI assembles the handlers, JWT middleware, and error-rendering policy, then
// mounts every route from the spec onto mux and returns it. It is the http
// package's single exported seam: the middleware, context key, and error helpers
// stay unexported here.
//
// Mounting happens here rather than in main because the two error hooks below
// render through writeJSONError, which is unexported by design.
func NewAPI(p NewAPIParams, mux openapi.ServeMux) http.Handler {
	s := &server{
		AuthHandler:         p.Auth,
		PrescriptionHandler: p.Prescription,
		PriceSearchHandler:  p.Search,
		OrderHandler:        p.Order,
		ShippingHandler:     p.Shipping,
	}
	protected := map[string]struct{}{
		"GetProfile":               {},
		"UploadPrescription":       {},
		"ListPrescriptions":        {},
		"SearchPrescriptionPrices": {},
		"CreateOrder":              {},
		"GetOrderStatus":           {},
	}
	authMW := newAuthMiddleware(p.TokenIssuer, protected)
	tracingMW := newTracingMiddleware()

	// Body-binding and handler errors, raised inside the strict handler.
	strictOpts := openapi.StrictHTTPServerOptions{
		RequestErrorHandlerFunc: func(
			w http.ResponseWriter,
			_ *http.Request,
			err error,
		) {
			writeJSONError(w, http.StatusBadRequest, err.Error())
		},
		ResponseErrorHandlerFunc: func(
			w http.ResponseWriter,
			r *http.Request,
			err error,
		) {
			p.Logger.ErrorContext(r.Context(), "unexpected handler error", "error", err)
			writeJSONError(w, http.StatusInternalServerError, msgInternalServerError)
		},
	}
	api := openapi.NewStrictHandlerWithOptions(s, []openapi.StrictMiddlewareFunc{authMW, tracingMW}, strictOpts)

	// Path- and query-parameter binding runs before the strict handler is reached,
	// so its failures surface on this separate hook. Without it the generated
	// default renders text/plain, breaking the spec's Error schema contract.
	apiHandler := openapi.HandlerWithOptions(api, openapi.StdHTTPServerOptions{
		BaseRouter: mux,
		ErrorHandlerFunc: func(
			w http.ResponseWriter,
			_ *http.Request,
			err error,
		) {
			writeJSONError(w, http.StatusBadRequest, err.Error())
		},
	})
	return newLoggingMiddleware(p.Logger)(apiHandler)
}
