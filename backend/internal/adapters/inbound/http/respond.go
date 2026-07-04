package http

import (
	"encoding/json"
	"net/http"

	"github.com/LoneWolfPR/MedMarket/backend/internal/adapters/inbound/http/openapi"
)

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(openapi.Error{Message: msg})
}
