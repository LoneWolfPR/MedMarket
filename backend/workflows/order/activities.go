package order

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/LoneWolfPR/MedMarket/backend/internal/adapters/outbound/mailer"
	"github.com/LoneWolfPR/MedMarket/backend/internal/adapters/outbound/shipping"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/order"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/pharmacy"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/outbound"

	"github.com/google/uuid"
	"go.temporal.io/sdk/temporal"
)

// Activities holds the properties necessary for execution of methods
type Activities struct {
	clients        map[string]outbound.PharmacyClient
	orderRepo      outbound.OrderRepository
	shippingClient outbound.ShippingClient
	emailSender    outbound.EmailSender
}

// NewActivitiesParams holds the necessary params to construct an instance
type NewActivitiesParams struct {
	Clients        map[string]outbound.PharmacyClient
	OrderRepo      outbound.OrderRepository
	ShippingClient outbound.ShippingClient
	EmailSender    outbound.EmailSender
}

// NewActivities constructs a new instance
func NewActivities(p NewActivitiesParams) *Activities {
	return &Activities{
		clients:        p.Clients,
		orderRepo:      p.OrderRepo,
		shippingClient: p.ShippingClient,
		emailSender:    p.EmailSender,
	}
}

// PlaceOrderActivityResult holds the result fields so they can be passed back to the workflow
type PlaceOrderActivityResult struct {
	PharmacyOrderID string
	TrackingID      string
	NewTotalCents   int64
	OrderStatus     string
}

// PlaceOrderActivity takes the input and places an order through the correct
// pharmacy adapter
func (a *Activities) PlaceOrderActivity(ctx context.Context, i Input) (*PlaceOrderActivityResult, error) {
	const invalidOrderErrLabel = "InvalidOrderInput"
	client := a.clients[i.PharmacyCode]
	if client == nil {
		return nil, temporal.NewNonRetryableApplicationError(
			fmt.Sprintf("client with code %s does not exist", i.PharmacyCode),
			"MissingPharmacyClient",
			nil,
		)
	}

	orderInput, err := pharmacy.NewOrderInput(pharmacy.NewOrderInputParams{
		IdempotencyKey: i.IdempotencyKey,
		PharmacyItemID: i.PharmacyItemID,
		Qty:            i.Qty,
		ShippingAddress: shared.Address{
			Street1: i.Address.Street1,
			Street2: i.Address.Street2,
			City:    i.Address.City,
			State:   i.Address.State,
			Zip:     i.Address.Zip,
		},
		RecipientName: i.RecipientName,
	})
	if err != nil {
		return nil, temporal.NewNonRetryableApplicationError(
			"invalid order input",
			invalidOrderErrLabel,
			err,
		)
	}
	result, err := client.PlaceOrder(ctx, orderInput)
	if err != nil {
		var poErr *outbound.PlaceOrderError
		if errors.As(err, &poErr) {
			return nil, temporal.NewNonRetryableApplicationError(
				"place order failed",
				string(poErr.Kind),
				err,
			)
		}
		return nil, fmt.Errorf("error placing order %s for item %s: %w",
			i.OrderID,
			i.PharmacyItemID, err)
	}

	return &PlaceOrderActivityResult{
		PharmacyOrderID: result.PharmacyOrderID(),
		TrackingID:      result.TrackingID(),
		NewTotalCents:   result.NewTotal().Cents(),
		OrderStatus:     result.OrderStatus(),
	}, nil
}

// UpdateOrderActivityInput mirrors the structure of the Order type for use
// in the activity
type UpdateOrderActivityInput struct {
	ID              uuid.UUID
	Status          *string
	PharmacyOrderID *string
	TrackingID      *string
	PricePaidCents  *int64
}

// UpdateOrderActivity updates an order record in the database
func (a *Activities) UpdateOrderActivity(ctx context.Context, i UpdateOrderActivityInput) error {
	const invalidUpdateOrderInputErr = "InvalidUpdateOrderInput"
	// Fetch current order record
	orderRecord, err := a.orderRepo.GetByID(ctx, i.ID)
	if err != nil {
		if errors.Is(err, outbound.ErrOrderNotFound) {
			return temporal.NewNonRetryableApplicationError(
				outbound.ErrOrderNotFound.Error(),
				"OrderNotFound",
				err,
			)
		}
		return fmt.Errorf("error getting order record: %w", err)
	}

	if i.Status != nil {
		if !slices.Contains(order.AcceptedStatuses, *i.Status) {
			return temporal.NewNonRetryableApplicationError(
				"invalid order status",
				invalidUpdateOrderInputErr,
				fmt.Errorf("status %q is invalid", *i.Status),
			)
		}
		orderRecord.Status = *i.Status
	}
	if i.PharmacyOrderID != nil {
		orderRecord.PharmacyOrderID = *i.PharmacyOrderID
	}
	if i.TrackingID != nil {
		orderRecord.TrackingID = *i.TrackingID
	}
	if i.PricePaidCents != nil {
		pricePaid, err := shared.NewMoneyFromCents(*i.PricePaidCents)
		if err != nil {
			return temporal.NewNonRetryableApplicationError(
				"invalid price paid",
				invalidUpdateOrderInputErr,
				err,
			)
		}
		orderRecord.PricePaid = &pricePaid
	}

	_, err = a.orderRepo.Update(ctx, orderRecord)
	if err != nil {
		return fmt.Errorf("error updating order: %w", err)
	}
	return nil
}

// RegisterWebhookActivityInput contains the properties necessary to
// register the callback
type RegisterWebhookActivityInput struct {
	TrackingID  string
	CallbackURL string
}

// RegisterWebhookActivity uses the outbound adapter to register a callback
// with the shipping service so shipping updates can be received
func (a *Activities) RegisterWebhookActivity(
	ctx context.Context,
	i RegisterWebhookActivityInput,
) error {
	const invalidRegisterWebhookInputErr = "InvalidRegisterWebhookInput"
	trimmedTracking := strings.TrimSpace(i.TrackingID)
	trimmedCallback := strings.TrimSpace(i.CallbackURL)
	if trimmedTracking == "" {
		return temporal.NewNonRetryableApplicationError(
			"tracking id is missing",
			invalidRegisterWebhookInputErr,
			errors.New("tracking id is missing"),
		)
	}
	if trimmedCallback == "" {
		return temporal.NewNonRetryableApplicationError(
			"callback url is missing",
			invalidRegisterWebhookInputErr,
			errors.New("callback url is missing"),
		)
	}

	err := a.shippingClient.RegisterWebhook(ctx, trimmedTracking, trimmedCallback)
	if err != nil {
		if errors.Is(err, shipping.ErrShippingRejected) {
			return temporal.NewNonRetryableApplicationError(
				"error registerring webhook",
				"ErrShippingRejected",
				err,
			)
		}
		return fmt.Errorf("error registerring webhook: %w", err)
	}
	return nil
}

// EmailUpdateInput contains the properties needed to construct
// a shipping update email
type EmailUpdateInput struct {
	To      string
	Message string
}

// SendEmailUpdate sends an email to a user with shipping update information
func (a *Activities) SendEmailUpdate(ctx context.Context, i EmailUpdateInput) error {
	const subject = "MedMarket Order Update"
	replyTo, _ := shared.NewEmail("medMarket@lonewolfdigital.com")
	const invalidSendEmailInputErr = "InvalidSendEmailInput"

	to, err := shared.NewEmail(i.To)
	if err != nil {
		return temporal.NewNonRetryableApplicationError(
			"error with to address",
			invalidSendEmailInputErr,
			err,
		)
	}

	if i.Message == "" {
		return temporal.NewNonRetryableApplicationError(
			"message is missing",
			invalidSendEmailInputErr,
			errors.New("message is missing"),
		)
	}

	err = a.emailSender.Send(ctx, outbound.EmailSenderParams{
		To:      to,
		ReplyTo: replyTo,
		Subject: subject,
		Message: i.Message,
	})
	if err != nil {
		if errors.Is(err, mailer.ErrInvalidMessage) {
			return temporal.NewNonRetryableApplicationError(
				mailer.ErrInvalidMessage.Error(),
				"ErrInvalidMessage",
				err,
			)
		}
		return fmt.Errorf("error sending email notification: %w", err)
	}
	return nil
}
