package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/order"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/inbound"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/outbound"
)

// OrderService is used to place orders
type OrderService struct {
	logger       *slog.Logger
	orderRepo    outbound.OrderRepository
	offerRepo    outbound.OfferRepository
	pharmRepo    outbound.PharmacyRepository
	rxRepo       outbound.PrescriptionRepository
	userRepo     outbound.UserRepository
	orderStarter outbound.OrderStarter
}

var _ inbound.OrderService = (*OrderService)(nil)

// NewOrderServiceParams holds the parameters necessary to construct the service
type NewOrderServiceParams struct {
	Logger       *slog.Logger
	OrderRepo    outbound.OrderRepository
	OfferRepo    outbound.OfferRepository
	PharmRepo    outbound.PharmacyRepository
	RxRepo       outbound.PrescriptionRepository
	UserRepo     outbound.UserRepository
	OrderStarter outbound.OrderStarter
}

// NewOrderService constructs the service
func NewOrderService(p NewOrderServiceParams) (*OrderService, error) {
	if p.Logger == nil {
		return nil, errors.New("logger is missing")
	}
	if p.OrderRepo == nil {
		return nil, errors.New("order repository is missing")
	}
	if p.OfferRepo == nil {
		return nil, errors.New("offer repository is missing")
	}
	if p.PharmRepo == nil {
		return nil, errors.New("pharmacy repository is missing")
	}
	if p.RxRepo == nil {
		return nil, errors.New("prescription repository is missing")
	}
	if p.UserRepo == nil {
		return nil, errors.New("user repository is missing")
	}
	if p.OrderStarter == nil {
		return nil, errors.New("order starter is missing")
	}
	return &OrderService{
		logger:       p.Logger,
		orderRepo:    p.OrderRepo,
		offerRepo:    p.OfferRepo,
		pharmRepo:    p.PharmRepo,
		rxRepo:       p.RxRepo,
		userRepo:     p.UserRepo,
		orderStarter: p.OrderStarter,
	}, nil
}

// PlaceOrder takes an incoming request from the external handler and calls out to the starter
// to initiate an order workflow
func (s *OrderService) PlaceOrder(ctx context.Context, i inbound.OrderInput) (inbound.OrderView, error) {
	// Fetch offer
	offerRecord, err := s.offerRepo.GetByID(ctx, i.OfferID)
	if err != nil {
		if errors.Is(err, outbound.ErrOfferNotFound) {
			return inbound.OrderView{}, fmt.Errorf("%w: %w", inbound.ErrOfferNotFound, err)
		}
		return inbound.OrderView{}, fmt.Errorf("error fetching offer record: %w", err)
	}
	// Fetch prescription
	rxRecord, err := s.rxRepo.GetByID(ctx, offerRecord.PrescriptionID)
	if err != nil {
		return inbound.OrderView{}, fmt.Errorf("error fetching prescription record: %w", err)
	}
	// Fetch user
	userRecord, err := s.userRepo.GetByID(ctx, i.UserID)
	if err != nil {
		if errors.Is(err, outbound.ErrUserNotFound) {
			return inbound.OrderView{}, fmt.Errorf("%w: %w", inbound.ErrInvalidCredentials, err)
		}
		return inbound.OrderView{}, fmt.Errorf("error fetching user record: %w", err)
	}
	// validate offer owned by user
	if rxRecord.UserID != i.UserID {
		return inbound.OrderView{}, inbound.ErrOfferNotFound
	}
	// validate offer is not expired
	if time.Now().After(offerRecord.ExpiresAt) {
		return inbound.OrderView{}, inbound.ErrOfferExpired
	}
	// Fetch pharmacy
	pharmRecord, err := s.pharmRepo.GetByID(ctx, offerRecord.Quote.PharmacyID())
	if err != nil {
		return inbound.OrderView{}, fmt.Errorf("error fetching pharmacy record: %w", err)
	}
	// Create Order Record
	newOrder, err := order.NewOrder(order.NewOrderParams{
		PrescriptionID: offerRecord.PrescriptionID,
		OfferID:        i.OfferID,
		Qty:            rxRecord.Qty,
	})
	if err != nil {
		return inbound.OrderView{}, fmt.Errorf("error creating order: %w", err)
	}

	orderRecord, err := s.orderRepo.Create(ctx, newOrder)
	if err != nil {
		if errors.Is(err, outbound.ErrOrderExists) {
			return inbound.OrderView{}, fmt.Errorf("%w: %w", inbound.ErrOrderAlreadyPlaced, err)
		}
		return inbound.OrderView{}, fmt.Errorf("error saving order record: %w", err)
	}

	totalCents := offerRecord.Quote.Price().Cents() * int64(rxRecord.Qty)
	orderRequestInput, err := order.NewPlacementRequest(order.NewPlacementRequestParams{
		OrderID:        orderRecord.ID,
		PharmacyCode:   pharmRecord.Code,
		PharmacyItemID: offerRecord.Quote.PharmacyItemID(),
		ItemName:       rxRecord.MedName,
		RecipientName:  userRecord.FirstName + " " + userRecord.LastName,
		RecipientEmail: userRecord.Email.String(),
		Qty:            orderRecord.Qty,
		Address:        userRecord.Address,
		PaymentMethod:  i.PaymentMethod,
		AmountCents:    totalCents,
	})
	if err != nil {
		return inbound.OrderView{}, fmt.Errorf("error creating request: %w", err)
	}

	if err = s.orderStarter.StartOrder(ctx, orderRequestInput); err != nil {
		orderRecord.Status = order.StatusFailed
		if _, updateErr := s.orderRepo.Update(ctx, orderRecord); updateErr != nil {
			s.logger.ErrorContext(
				ctx,
				"error updating order record after failed start",
				"order id", orderRecord.ID,
				"error", updateErr)
		}
		return inbound.OrderView{}, fmt.Errorf("error starting order: %w", err)
	}

	return inbound.OrderView{
		OrderID:  orderRecord.ID,
		ItemName: rxRecord.MedName,
		Status:   orderRecord.Status,
		Qty:      orderRecord.Qty,
	}, nil
}
