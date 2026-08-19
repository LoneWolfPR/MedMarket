package postgres

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/LoneWolfPR/MedMarket/backend/ent"
	entorder "github.com/LoneWolfPR/MedMarket/backend/ent/order"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/order"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/outbound"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ptr"

	"github.com/google/uuid"
)

// OrderRepository handles all managing of order data
type OrderRepository struct {
	client *ent.Client
	logger *slog.Logger
}

var _ outbound.OrderRepository = (*OrderRepository)(nil)

// NewOrderRepositoryParams holds the input params needed to construct an instance
type NewOrderRepositoryParams struct {
	Client *ent.Client
	Logger *slog.Logger
}

// NewOrderRepository constructs an instance
func NewOrderRepository(p NewOrderRepositoryParams) (*OrderRepository, error) {
	if p.Client == nil {
		return nil, errors.New("client is missing")
	}
	if p.Logger == nil {
		return nil, errors.New("logger is missing")
	}
	return &OrderRepository{
		client: p.Client,
		logger: p.Logger,
	}, nil
}

// Create inserts a new order record in the database
func (r *OrderRepository) Create(ctx context.Context, o *order.Order) (*order.Order, error) {
	newOrderRecord, err := r.client.Order.Create().
		SetPrescriptionID(o.PrescriptionID).
		SetOfferID(o.OfferID).
		SetStatus(o.Status).
		SetQuantity(o.Qty).
		Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, outbound.ErrOrderExists
		}
		return nil, fmt.Errorf("error creating order record: %w", err)
	}
	r.logger.DebugContext(
		ctx,
		"new order record created",
		"prescription_id",
		newOrderRecord.PrescriptionID,
		"order_id",
		newOrderRecord.ID,
	)
	return mapToDomainOrder(newOrderRecord)
}

// Update edits a record in the database
func (r *OrderRepository) Update(ctx context.Context, o *order.Order) (*order.Order, error) {
	updateQuery := r.client.Order.UpdateOneID(o.ID).
		SetStatus(o.Status).
		SetPharmacyOrderID(o.PharmacyOrderID).
		SetTrackingID(o.TrackingID)
	if o.PricePaid != nil {
		updateQuery.SetPricePaidCents(o.PricePaid.Cents())
	}
	updatedRecord, err := updateQuery.Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, outbound.ErrOrderNotFound
		}
		return nil, fmt.Errorf("error updating record with id %s: %w", o.ID, err)
	}
	r.logger.DebugContext(
		ctx,
		"order record updated",
		"order_id",
		updatedRecord.ID,
	)
	return mapToDomainOrder(updatedRecord)
}

// GetByID queries the database for a specific order record
func (r *OrderRepository) GetByID(ctx context.Context, id uuid.UUID) (*order.Order, error) {
	orderRecord, err := r.client.Order.Query().
		Where(entorder.IDEQ(id)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, outbound.ErrOrderNotFound
		}
		return nil, fmt.Errorf("error fetching order record: %w", err)
	}
	return mapToDomainOrder(orderRecord)
}

// List queries the database for orders that match a list of prescription IDs
func (r *OrderRepository) List(ctx context.Context, rxIDs []uuid.UUID) ([]order.Order, error) {
	orders := []order.Order{}
	if len(rxIDs) == 0 {
		return orders, nil
	}
	records, err := r.client.Order.Query().
		Where(entorder.PrescriptionIDIn(rxIDs...)).
		Order(ent.Desc(entorder.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("error fetching order records: %w", err)
	}
	for _, record := range records {
		orderInfo, err := mapToDomainOrder(record)
		if err != nil {
			r.logger.ErrorContext(ctx, "error mapping order record", "order_id", record.ID, "error", err)
		} else {
			orders = append(orders, *orderInfo)
		}
	}
	return orders, nil
}

func mapToDomainOrder(orderRecord *ent.Order) (*order.Order, error) {
	var pricePaid *shared.Money
	if orderRecord.PricePaidCents != nil {
		recordCents := ptr.Deref(orderRecord.PricePaidCents)
		money, err := shared.NewMoneyFromCents(recordCents)
		if err != nil {
			return nil, fmt.Errorf("error with price on order record: %w", err)
		}
		pricePaid = ptr.To(money)
	}

	return &order.Order{
		ID:              orderRecord.ID,
		PrescriptionID:  orderRecord.PrescriptionID,
		OfferID:         orderRecord.OfferID,
		Status:          orderRecord.Status,
		PharmacyOrderID: orderRecord.PharmacyOrderID,
		TrackingID:      orderRecord.TrackingID,
		Qty:             orderRecord.Quantity,
		PricePaid:       pricePaid,
		PlacedAt:        orderRecord.CreatedAt,
	}, nil
}
