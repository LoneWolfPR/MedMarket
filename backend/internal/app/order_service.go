package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/order"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/pharmacy"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/prescription"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/inbound"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/outbound"

	"github.com/google/uuid"
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
	querier      outbound.OrderStatusQuerier
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
	Querier      outbound.OrderStatusQuerier
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
	if p.Querier == nil {
		return nil, errors.New("order status querier is missing")
	}
	return &OrderService{
		logger:       p.Logger,
		orderRepo:    p.OrderRepo,
		offerRepo:    p.OfferRepo,
		pharmRepo:    p.PharmRepo,
		rxRepo:       p.RxRepo,
		userRepo:     p.UserRepo,
		orderStarter: p.OrderStarter,
		querier:      p.Querier,
	}, nil
}

// ownedOffer resolves an offer and its prescription, then verifies the caller owns
// them. Every way that chain can fail — the offer is gone, its prescription is gone,
// or the prescription belongs to someone else — collapses to inbound.ErrOfferNotFound
// so a caller can never probe for records it has no right to see. Callers translate
// that sentinel into their own vocabulary.
func (s *OrderService) ownedOffer(
	ctx context.Context,
	userID, offerID uuid.UUID,
) (*pharmacy.Offer, *prescription.Prescription, error) {
	offerRecord, err := s.offerRepo.GetByID(ctx, offerID)
	if err != nil {
		if errors.Is(err, outbound.ErrOfferNotFound) {
			return nil, nil, fmt.Errorf("%w: %w", inbound.ErrOfferNotFound, err)
		}
		return nil, nil, fmt.Errorf("error fetching offer record: %w", err)
	}

	rxRecord, err := s.rxRepo.GetByID(ctx, offerRecord.PrescriptionID)
	if err != nil {
		if errors.Is(err, outbound.ErrPrescriptionNotFound) {
			// A foreign key guarantees this row exists, so its absence is corruption
			// rather than a client error. The caller still sees not-found, but the
			// integrity problem must not disappear with it.
			s.logger.ErrorContext(ctx, "offer references a missing prescription",
				"offer_id", offerID,
				"prescription_id", offerRecord.PrescriptionID)
			return nil, nil, fmt.Errorf("%w: %w", inbound.ErrOfferNotFound, err)
		}
		return nil, nil, fmt.Errorf("error fetching prescription record: %w", err)
	}

	if rxRecord.UserID != userID {
		return nil, nil, inbound.ErrOfferNotFound
	}
	return offerRecord, rxRecord, nil
}

// PlaceOrder takes an incoming request from the external handler and calls out to the starter
// to initiate an order workflow
func (s *OrderService) PlaceOrder(ctx context.Context, i inbound.OrderInput) (inbound.OrderView, error) {
	// Fetch the offer and prescription, confirming the caller owns them
	offerRecord, rxRecord, err := s.ownedOffer(ctx, i.UserID, i.OfferID)
	if err != nil {
		return inbound.OrderView{}, err
	}
	// Fetch user
	userRecord, err := s.userRepo.GetByID(ctx, i.UserID)
	if err != nil {
		if errors.Is(err, outbound.ErrUserNotFound) {
			return inbound.OrderView{}, fmt.Errorf("%w: %w", inbound.ErrInvalidCredentials, err)
		}
		return inbound.OrderView{}, fmt.Errorf("error fetching user record: %w", err)
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
				"order_id", orderRecord.ID,
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

// GetOrderStatus fetches the order status from the order record and the shipping status
// if there is an active order workflow
func (s *OrderService) GetOrderStatus(
	ctx context.Context,
	userID, orderID uuid.UUID,
) (inbound.OrderStatusView, error) {
	orderRecord, err := s.orderRepo.GetByID(ctx, orderID)
	if err != nil {
		if errors.Is(err, outbound.ErrOrderNotFound) {
			return inbound.OrderStatusView{}, inbound.ErrOrderNotFound
		}
		return inbound.OrderStatusView{}, fmt.Errorf("error fetching order status: %w", err)
	}
	// An order the caller doesn't own is reported as missing, in the order's vocabulary
	if _, _, err = s.ownedOffer(ctx, userID, orderRecord.OfferID); err != nil {
		if errors.Is(err, inbound.ErrOfferNotFound) {
			return inbound.OrderStatusView{}, inbound.ErrOrderNotFound
		}
		return inbound.OrderStatusView{}, err
	}

	shippingStatus, err := s.querier.QueryShippingStatus(ctx, orderID)
	if err != nil {
		if !errors.Is(err, outbound.ErrOrderWorkflowNotFound) {
			s.logger.ErrorContext(ctx, "error querying shipping status", "error", err)
		}
	}
	return inbound.OrderStatusView{
		OrderID:        orderID,
		Status:         orderRecord.Status,
		ShippingStatus: shippingStatus,
	}, nil

}

// ListOrders returns a summary of every order belonging to the caller. Ownership is
// enforced by construction: orders are scoped to the prescriptions fetched for this
// user, so nothing outside that set is reachable.
func (s *OrderService) ListOrders(
	ctx context.Context,
	userID uuid.UUID,
) ([]inbound.OrderSummaryView, error) {
	rxRecords, err := s.rxRepo.List(ctx, userID)
	if err != nil {
		return []inbound.OrderSummaryView{}, fmt.Errorf("error fetching prescriptions for orders: %w", err)
	}
	rxList := make(map[uuid.UUID]string)
	rxIDs := []uuid.UUID{}
	for _, record := range rxRecords {
		rxList[record.ID] = record.MedName
		rxIDs = append(rxIDs, record.ID)
	}

	orderRecords, err := s.orderRepo.List(ctx, rxIDs)
	if err != nil {
		return []inbound.OrderSummaryView{}, fmt.Errorf("error fetching orders: %w", err)
	}
	summaries := []inbound.OrderSummaryView{}
	for _, orderRecord := range orderRecords {
		summaries = append(summaries, inbound.OrderSummaryView{
			OrderID:   orderRecord.ID,
			ItemName:  rxList[orderRecord.PrescriptionID],
			Qty:       orderRecord.Qty,
			Status:    orderRecord.Status,
			PlacedAt:  orderRecord.PlacedAt,
			PricePaid: orderRecord.PricePaid,
		})
	}
	return summaries, nil
}
