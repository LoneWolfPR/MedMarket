package http

import (
	"errors"
	"log/slog"

	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/inbound"
)

// AuthHandler is the adapter for processing all auth calls
type AuthHandler struct {
	logger *slog.Logger
	svc    inbound.UserService
}

// NewAuthHandlerParams is the object holding needed params
type NewAuthHandlerParams struct {
	Logger *slog.Logger
	Svc    inbound.UserService
}

// NewAuthHandler is the constructor to set up auth
func NewAuthHandler(p NewAuthHandlerParams) (*AuthHandler, error) {
	if p.Logger == nil {
		return nil, errors.New("logger is missing")
	}
	return &AuthHandler{
		logger: p.Logger,
		svc:    p.Svc,
	}, nil
}
