package http

import (
	"context"
	"errors"
	"log/slog"

	"github.com/LoneWolfPR/MedMarket/backend/internal/adapters/inbound/http/openapi"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/inbound"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ptr"
)

// AuthHandler is the adapter for processing all auth calls
type AuthHandler struct {
	logger *slog.Logger
	svc    inbound.UserService
}

var _ openapi.StrictServerInterface = (*AuthHandler)(nil)

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

// RegisterUser handles POST /api/auth/register, creating a new user account and
// returning the created profile.
func (h *AuthHandler) RegisterUser(
	ctx context.Context,
	req openapi.RegisterUserRequestObject,
) (openapi.RegisterUserResponseObject, error) {
	userRegInput := inbound.RegisterInput{
		FirstName: req.Body.FirstName,
		LastName:  req.Body.LastName,
		Email:     string(req.Body.Email),
		Password:  req.Body.Password,
		Phone:     ptr.Deref(req.Body.Phone),
		Address:   MapToSharedAddress(req.Body.Address),
	}

	newUser, err := h.svc.Register(ctx, userRegInput)
	if err != nil {
		switch {
		case errors.Is(err, inbound.ErrValidation):
			return openapi.RegisterUser400JSONResponse{
				BadRequestJSONResponse: openapi.BadRequestJSONResponse{
					Message: inbound.ErrValidation.Error(),
				},
			}, nil
		case errors.Is(err, inbound.ErrEmailTaken):
			return openapi.RegisterUser409JSONResponse{
				Message: inbound.ErrEmailTaken.Error(),
			}, nil
		default:
			return nil, err
		}

	}

	return openapi.RegisterUser201JSONResponse(toUserResponse(newUser)), nil
}

// LoginUser handles POST /api/auth/login, exchanging valid credentials for a
// bearer token.
func (h *AuthHandler) LoginUser(
	ctx context.Context,
	req openapi.LoginUserRequestObject,
) (openapi.LoginUserResponseObject, error) {
	token, err := h.svc.Login(
		ctx,
		string(req.Body.Email),
		req.Body.Password,
	)
	if err != nil {
		if errors.Is(err, inbound.ErrInvalidCredentials) {
			return openapi.LoginUser401JSONResponse{
				UnauthorizedJSONResponse: openapi.UnauthorizedJSONResponse{
					Message: inbound.ErrInvalidCredentials.Error(),
				},
			}, nil
		}
		return nil, err
	}

	return openapi.LoginUser200JSONResponse{Token: token}, nil
}

// GetProfile handles GET /api/auth/profile, returning the authenticated user's
// profile using the user ID injected into the context by the auth middleware.
func (h *AuthHandler) GetProfile(
	ctx context.Context,
	_ openapi.GetProfileRequestObject,
) (openapi.GetProfileResponseObject, error) {
	id, ok := getUserIDKeyValue(ctx)
	if !ok {
		return unauthorizedResponse(), nil
	}

	profile, err := h.svc.GetProfile(ctx, id)
	if err != nil {
		if errors.Is(err, inbound.ErrInvalidCredentials) {
			return unauthorizedResponse(), nil
		}
		return nil, err
	}
	return openapi.GetProfile200JSONResponse(toUserResponse(profile)), nil

}

func unauthorizedResponse() openapi.GetProfileResponseObject {
	return openapi.GetProfile401JSONResponse{
		UnauthorizedJSONResponse: openapi.UnauthorizedJSONResponse{
			Message: msgUnauthorized,
		},
	}
}
