package http

import (
	"context"
	"errors"
	"log/slog"

	"github.com/LoneWolfPR/MedMarket/backend/internal/adapters/inbound/http/openapi"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/inbound"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ptr"
)

// OrderHandler is the adapter for handling incoming orders
type OrderHandler struct {
	logger *slog.Logger
	svc    inbound.OrderService
}

// NewOrderHandlerParams holds the params necessary to construct the adapter
type NewOrderHandlerParams struct {
	Logger *slog.Logger
	Svc    inbound.OrderService
}

// NewOrderHandler constructs an instance of the adapter
func NewOrderHandler(p NewOrderHandlerParams) (*OrderHandler, error) {
	if p.Logger == nil {
		return nil, errors.New("logger is missing")
	}
	if p.Svc == nil {
		return nil, errors.New("service is missing")
	}
	return &OrderHandler{
		logger: p.Logger,
		svc:    p.Svc,
	}, nil
}

// CreateOrder parses the input and starts the process of placing an order
func (h *OrderHandler) CreateOrder(
	ctx context.Context,
	req openapi.CreateOrderRequestObject,
) (openapi.CreateOrderResponseObject, error) {
	userID, ok := getUserIDKeyValue(ctx)
	if !ok {
		return openapi.CreateOrder401JSONResponse{
			UnauthorizedJSONResponse: unauthorizedBody(),
		}, nil
	}

	orderInput := inbound.OrderInput{
		UserID:        userID,
		OfferID:       req.Body.OfferId,
		PaymentMethod: req.Body.PaymentMethod,
	}
	orderView, err := h.svc.PlaceOrder(ctx, orderInput)
	if err != nil {
		switch {
		case errors.Is(err, inbound.ErrOfferNotFound):
			return openapi.CreateOrder404JSONResponse{
				NotFoundJSONResponse: openapi.NotFoundJSONResponse{
					Message: inbound.ErrOfferNotFound.Error(),
				},
			}, nil
		case errors.Is(err, inbound.ErrOrderAlreadyPlaced):
			return openapi.CreateOrder409JSONResponse{
				ConflictJSONResponse: openapi.ConflictJSONResponse{
					Message: inbound.ErrOrderAlreadyPlaced.Error(),
				},
			}, nil
		case errors.Is(err, inbound.ErrOfferExpired):
			return openapi.CreateOrder410JSONResponse{
				GoneJSONResponse: openapi.GoneJSONResponse{
					Message: inbound.ErrOfferExpired.Error(),
				},
			}, nil
		// The user id came from a verified token, so a missing user means the
		// account was deleted while the token is still live — an auth failure,
		// not a server fault.
		case errors.Is(err, inbound.ErrInvalidCredentials):
			return openapi.CreateOrder401JSONResponse{
				UnauthorizedJSONResponse: unauthorizedBody(),
			}, nil
		default:
			return nil, err
		}
	}
	resp := openapi.OrderResponse{
		ItemName: orderView.ItemName,
		OrderId:  orderView.OrderID,
		Quantity: orderView.Qty,
		Status:   orderView.Status,
	}
	return openapi.CreateOrder201JSONResponse(resp), nil
}

// GetOrderStatus takes an order id and fetches the current status
func (h *OrderHandler) GetOrderStatus(
	ctx context.Context,
	req openapi.GetOrderStatusRequestObject,
) (openapi.GetOrderStatusResponseObject, error) {
	userID, ok := getUserIDKeyValue(ctx)
	if !ok {
		return openapi.GetOrderStatus401JSONResponse{
			UnauthorizedJSONResponse: unauthorizedBody(),
		}, nil
	}

	orderStatusView, err := h.svc.GetOrderStatus(ctx, userID, req.Id)
	if err != nil {
		if errors.Is(err, inbound.ErrOrderNotFound) {
			return openapi.GetOrderStatus404JSONResponse{
				NotFoundJSONResponse: openapi.NotFoundJSONResponse{
					Message: inbound.ErrOrderNotFound.Error(),
				},
			}, nil
		}
		return nil, err
	}

	resp := openapi.OrderStatusResponse{
		OrderId: req.Id,
		Status:  orderStatusView.Status,
	}
	if orderStatusView.ShippingStatus != "" {
		resp.ShippingStatus = ptr.To(orderStatusView.ShippingStatus)
	}

	return openapi.GetOrderStatus200JSONResponse(resp), nil
}

// ListOrders returns the authenticated user's orders, newest first
func (h *OrderHandler) ListOrders(
	ctx context.Context,
	_ openapi.ListOrdersRequestObject,
) (openapi.ListOrdersResponseObject, error) {
	userID, ok := getUserIDKeyValue(ctx)
	if !ok {
		return openapi.ListOrders401JSONResponse{
			UnauthorizedJSONResponse: unauthorizedBody(),
		}, nil
	}

	orderSummaryViews, err := h.svc.ListOrders(ctx, userID)
	if err != nil {
		return nil, err
	}

	orderList := []openapi.OrderSummaryResponse{}
	for _, summary := range orderSummaryViews {
		orderSummary := openapi.OrderSummaryResponse{
			ItemName: summary.ItemName,
			OrderId:  summary.OrderID,
			PlacedAt: summary.PlacedAt,
			Quantity: summary.Qty,
			Status:   summary.Status,
		}
		if summary.PricePaid != nil {
			orderSummary.PricePaidCents = ptr.To(summary.PricePaid.Cents())
		}
		orderList = append(orderList, orderSummary)
	}
	resp := openapi.OrderListResponse{
		Orders: orderList,
	}
	return openapi.ListOrders200JSONResponse(resp), nil
}
