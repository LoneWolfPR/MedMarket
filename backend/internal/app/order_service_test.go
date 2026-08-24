package app_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LoneWolfPR/MedMarket/backend/internal/app"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/order"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/pharmacy"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/prescription"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/user"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/inbound"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/outbound"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ptr"
)

// --- order-specific fakes (the shared ones live in fakes_test.go) -----------

type fakeOrderRepo struct {
	createFn  func(ctx context.Context, o *order.Order) (*order.Order, error)
	updateFn  func(ctx context.Context, o *order.Order) (*order.Order, error)
	getByIDFn func(ctx context.Context, id uuid.UUID) (*order.Order, error)
	listFn    func(ctx context.Context, rxIDs []uuid.UUID) ([]order.Order, error)
}

func (f fakeOrderRepo) Create(ctx context.Context, o *order.Order) (*order.Order, error) {
	return f.createFn(ctx, o)
}

func (f fakeOrderRepo) Update(ctx context.Context, o *order.Order) (*order.Order, error) {
	return f.updateFn(ctx, o)
}

func (f fakeOrderRepo) GetByID(ctx context.Context, id uuid.UUID) (*order.Order, error) {
	return f.getByIDFn(ctx, id)
}

func (f fakeOrderRepo) List(ctx context.Context, rxIDs []uuid.UUID) ([]order.Order, error) {
	return f.listFn(ctx, rxIDs)
}

type fakeOrderStarter struct {
	startFn func(ctx context.Context, i order.PlacementRequest) error
}

func (f fakeOrderStarter) StartOrder(ctx context.Context, i order.PlacementRequest) error {
	return f.startFn(ctx, i)
}

var (
	_ outbound.OrderRepository = fakeOrderRepo{}
	_ outbound.OrderStarter    = fakeOrderStarter{}
)

// --- helpers ----------------------------------------------------------------

func buildOffer(t *testing.T, rxID uuid.UUID, quote pharmacy.PriceQuote, expiresAt time.Time) *pharmacy.Offer {
	t.Helper()
	o, err := pharmacy.NewOffer(pharmacy.NewOfferParams{
		Quote:          quote,
		ExpiresAt:      expiresAt,
		PrescriptionID: rxID,
	})
	require.NoError(t, err)
	o.ID = uuid.New()
	return o
}

func happyOffer(o *pharmacy.Offer) fakeOfferRepo {
	return fakeOfferRepo{
		getByIDFn: func(context.Context, uuid.UUID) (*pharmacy.Offer, error) { return o, nil },
	}
}

func happyRx(rx prescription.Prescription) fakePrescriptionRepo {
	return fakePrescriptionRepo{
		getByIDFn: func(context.Context, uuid.UUID) (*prescription.Prescription, error) { return &rx, nil },
	}
}

func happyUser(u *user.User) fakeUserRepo {
	return fakeUserRepo{
		getByIDFn: func(context.Context, uuid.UUID) (*user.User, error) { return u, nil },
	}
}

func happyPharm(p pharmacy.Pharmacy) fakePharmacyRepo {
	return fakePharmacyRepo{
		getByIDFn: func(context.Context, uuid.UUID) (*pharmacy.Pharmacy, error) { return &p, nil },
	}
}

type fakeOrderStatusQuerier struct {
	queryFn func(ctx context.Context, orderID uuid.UUID) (string, error)
}

func (f fakeOrderStatusQuerier) QueryShippingStatus(ctx context.Context, orderID uuid.UUID) (string, error) {
	return f.queryFn(ctx, orderID)
}

var _ outbound.OrderStatusQuerier = fakeOrderStatusQuerier{}

func newOrderSvc(
	t *testing.T,
	offer fakeOfferRepo,
	rx fakePrescriptionRepo,
	usr fakeUserRepo,
	pharm fakePharmacyRepo,
	ord fakeOrderRepo,
	starter fakeOrderStarter,
	querier fakeOrderStatusQuerier,
) *app.OrderService {
	t.Helper()
	svc, err := app.NewOrderService(app.NewOrderServiceParams{
		Logger:       slog.New(slog.DiscardHandler),
		OrderRepo:    ord,
		OfferRepo:    offer,
		PharmRepo:    pharm,
		RxRepo:       rx,
		UserRepo:     usr,
		OrderStarter: starter,
		Querier:      querier,
	})
	require.NoError(t, err)
	return svc
}

// orderKit wires an all-succeeding PlaceOrder setup against consistent fixtures.
// A test swaps a single fake to exercise one failure branch, and reads gotReq /
// updated to assert what the service handed downstream.
type orderKit struct {
	input   inbound.OrderInput
	rxID    uuid.UUID
	quote   pharmacy.PriceQuote
	offer   fakeOfferRepo
	rx      fakePrescriptionRepo
	usr     fakeUserRepo
	pharm   fakePharmacyRepo
	ord     fakeOrderRepo
	starter fakeOrderStarter
	querier fakeOrderStatusQuerier
	gotReq  *order.PlacementRequest
	updated *order.Order
}

func newOrderKit(t *testing.T) *orderKit {
	t.Helper()

	userID := uuid.New()
	rx := buildRx(t, userID) // Qty 30, MedName "atorvastatin"
	pharmID := uuid.New()
	quote := buildQuote(t, pharmID, "sku-1", 1299)
	offer := buildOffer(t, rx.ID, quote, time.Now().Add(time.Hour))
	usr := makeStoredUser(t, "jane@example.com") // FirstName "Jane", LastName "Doe"
	usr.ID = userID
	usr.Address = shared.Address{Street1: "100 Market St", City: "Springfield", State: "IL", Zip: "62704"}
	pharm := buildPharmacy(t, pharmID, "mock-pharmacy-a", "Mock Pharmacy A")

	k := &orderKit{
		input: inbound.OrderInput{UserID: userID, OfferID: offer.ID, PaymentMethod: "pm_card_visa"},
		rxID:  rx.ID,
		quote: quote,
		offer: happyOffer(offer),
		rx:    happyRx(rx),
		usr:   happyUser(usr),
		pharm: happyPharm(pharm),
	}
	k.ord = fakeOrderRepo{
		// Stand in for the DB assigning a primary key on insert.
		createFn: func(_ context.Context, o *order.Order) (*order.Order, error) {
			saved := *o
			saved.ID = uuid.New()
			return &saved, nil
		},
		updateFn: func(_ context.Context, o *order.Order) (*order.Order, error) {
			k.updated = o
			return o, nil
		},
	}
	k.starter = fakeOrderStarter{
		startFn: func(_ context.Context, i order.PlacementRequest) error {
			captured := i
			k.gotReq = &captured
			return nil
		},
	}
	return k
}

func (k *orderKit) svc(t *testing.T) *app.OrderService {
	t.Helper()
	return newOrderSvc(t, k.offer, k.rx, k.usr, k.pharm, k.ord, k.starter, k.querier)
}

// --- NewOrderService --------------------------------------------------------

func TestNewOrderService_Validation(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	base := app.NewOrderServiceParams{
		Logger:       logger,
		OrderRepo:    fakeOrderRepo{},
		OfferRepo:    fakeOfferRepo{},
		PharmRepo:    fakePharmacyRepo{},
		RxRepo:       fakePrescriptionRepo{},
		UserRepo:     fakeUserRepo{},
		OrderStarter: fakeOrderStarter{},
		Querier:      fakeOrderStatusQuerier{},
	}

	_, err := app.NewOrderService(base)
	require.NoError(t, err, "the baseline params must be valid")

	tests := map[string]func(p *app.NewOrderServiceParams){
		"missing logger":        func(p *app.NewOrderServiceParams) { p.Logger = nil },
		"missing order repo":    func(p *app.NewOrderServiceParams) { p.OrderRepo = nil },
		"missing offer repo":    func(p *app.NewOrderServiceParams) { p.OfferRepo = nil },
		"missing pharmacy repo": func(p *app.NewOrderServiceParams) { p.PharmRepo = nil },
		"missing rx repo":       func(p *app.NewOrderServiceParams) { p.RxRepo = nil },
		"missing user repo":     func(p *app.NewOrderServiceParams) { p.UserRepo = nil },
		"missing order starter": func(p *app.NewOrderServiceParams) { p.OrderStarter = nil },
		"missing querier":       func(p *app.NewOrderServiceParams) { p.Querier = nil },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			p := base
			mutate(&p)
			_, err := app.NewOrderService(p)
			require.Error(t, err)
		})
	}
}

// --- PlaceOrder -------------------------------------------------------------

func TestOrderService_PlaceOrder_HappyPath(t *testing.T) {
	k := newOrderKit(t)

	view, err := k.svc(t).PlaceOrder(context.Background(), k.input)
	require.NoError(t, err)

	// The view returned to the caller.
	assert.NotEqual(t, uuid.Nil, view.OrderID)
	assert.Equal(t, "atorvastatin", view.ItemName)
	assert.Equal(t, 30, view.Qty)
	assert.Equal(t, order.StatusPlaced, view.Status)

	// The placement request handed to the workflow starter: fields assembled from
	// the offer, prescription, user, and pharmacy, with the amount = price x qty
	// and the caller's payment method threaded through.
	require.NotNil(t, k.gotReq)
	assert.Equal(t, view.OrderID, k.gotReq.OrderID())
	assert.Equal(t, "mock-pharmacy-a", k.gotReq.PharmacyCode())
	assert.Equal(t, "sku-1", k.gotReq.PharmacyItemID())
	assert.Equal(t, "atorvastatin", k.gotReq.ItemName())
	assert.Equal(t, "Jane Doe", k.gotReq.RecipientName())
	assert.Equal(t, "jane@example.com", k.gotReq.RecipientEmail())
	assert.Equal(t, 30, k.gotReq.Qty())
	assert.Equal(t, "pm_card_visa", k.gotReq.PaymentMethod())
	assert.Equal(t, int64(1299*30), k.gotReq.AmountCents())
}

func TestOrderService_PlaceOrder_OfferNotFound(t *testing.T) {
	k := newOrderKit(t)
	k.offer = fakeOfferRepo{
		getByIDFn: func(context.Context, uuid.UUID) (*pharmacy.Offer, error) {
			return nil, outbound.ErrOfferNotFound
		},
	}

	_, err := k.svc(t).PlaceOrder(context.Background(), k.input)
	require.ErrorIs(t, err, inbound.ErrOfferNotFound)
}

func TestOrderService_PlaceOrder_OfferFetchErrorIsGeneric(t *testing.T) {
	// An unexpected offer-repo error is not a typed 4xx; it propagates as-is to a 500.
	k := newOrderKit(t)
	k.offer = fakeOfferRepo{
		getByIDFn: func(context.Context, uuid.UUID) (*pharmacy.Offer, error) { return nil, errBoom },
	}

	_, err := k.svc(t).PlaceOrder(context.Background(), k.input)
	require.ErrorIs(t, err, errBoom)
	assert.NotErrorIs(t, err, inbound.ErrOfferNotFound)
}

func TestOrderService_PlaceOrder_PrescriptionFetchErrorIsGeneric(t *testing.T) {
	// An unexpected prescription-repo error is not a typed 4xx; it propagates to a 500.
	k := newOrderKit(t)
	k.rx = fakePrescriptionRepo{
		getByIDFn: func(context.Context, uuid.UUID) (*prescription.Prescription, error) { return nil, errBoom },
	}

	_, err := k.svc(t).PlaceOrder(context.Background(), k.input)
	require.ErrorIs(t, err, errBoom)
}

func TestOrderService_PlaceOrder_PrescriptionNotFoundHidesOffer(t *testing.T) {
	// A broken ownership chain collapses to not-found rather than leaking that the
	// offer exists — matching GetOrderStatus. FKs make this state impossible, so the
	// service logs it as corruption even though the caller only sees a 404.
	k := newOrderKit(t)
	k.rx = fakePrescriptionRepo{
		getByIDFn: func(context.Context, uuid.UUID) (*prescription.Prescription, error) {
			return nil, outbound.ErrPrescriptionNotFound
		},
	}

	_, err := k.svc(t).PlaceOrder(context.Background(), k.input)
	require.ErrorIs(t, err, inbound.ErrOfferNotFound)
}

func TestOrderService_PlaceOrder_UserNotFoundMapsToInvalidCredentials(t *testing.T) {
	// The user id comes from the JWT, so a missing user is a deleted account with a
	// live token, not a bad request.
	k := newOrderKit(t)
	k.usr = fakeUserRepo{
		getByIDFn: func(context.Context, uuid.UUID) (*user.User, error) { return nil, outbound.ErrUserNotFound },
	}

	_, err := k.svc(t).PlaceOrder(context.Background(), k.input)
	require.ErrorIs(t, err, inbound.ErrInvalidCredentials)
}

func TestOrderService_PlaceOrder_OwnershipMismatchHidesExistence(t *testing.T) {
	// An offer whose prescription belongs to someone else collapses to not-found so
	// existence is never leaked.
	k := newOrderKit(t)
	k.rx = happyRx(buildRx(t, uuid.New())) // owned by a different user

	_, err := k.svc(t).PlaceOrder(context.Background(), k.input)
	require.ErrorIs(t, err, inbound.ErrOfferNotFound)
}

func TestOrderService_PlaceOrder_ExpiredOffer(t *testing.T) {
	k := newOrderKit(t)
	k.offer = happyOffer(buildOffer(t, k.rxID, k.quote, time.Now().Add(-time.Hour)))

	_, err := k.svc(t).PlaceOrder(context.Background(), k.input)
	require.ErrorIs(t, err, inbound.ErrOfferExpired)
}

// userWithAddress rebuilds the kit's user carrying the given address, so a test
// can vary only the field under examination.
func (k *orderKit) userWithAddress(t *testing.T, addr shared.Address) fakeUserRepo {
	t.Helper()
	usr := makeStoredUser(t, "jane@example.com")
	usr.ID = k.input.UserID
	usr.Address = addr
	return happyUser(usr)
}

func TestOrderService_PlaceOrder_InvalidAddress(t *testing.T) {
	// Street2 is the only optional field, so dropping any of the other four is
	// enough on its own to fail the gate.
	tests := map[string]shared.Address{
		"no address at all": {},
		"missing street1":   {City: "Springfield", State: "IL", Zip: "62704"},
		"missing city":      {Street1: "100 Market St", State: "IL", Zip: "62704"},
		"missing state":     {Street1: "100 Market St", City: "Springfield", Zip: "62704"},
		"missing zip":       {Street1: "100 Market St", City: "Springfield", State: "IL"},
	}
	for name, addr := range tests {
		t.Run(name, func(t *testing.T) {
			k := newOrderKit(t)
			k.usr = k.userWithAddress(t, addr)

			_, err := k.svc(t).PlaceOrder(context.Background(), k.input)
			require.ErrorIs(t, err, inbound.ErrNoValidAddress)
		})
	}
}

func TestOrderService_PlaceOrder_InvalidAddressWritesNothing(t *testing.T) {
	// The gate is a precondition, not a late failure. Running it after the insert
	// would leave an orphaned order that the partial unique index then treats as
	// active, permanently answering 409 for every retry of the same offer.
	k := newOrderKit(t)
	k.usr = k.userWithAddress(t, shared.Address{})

	var created bool
	k.ord = fakeOrderRepo{
		createFn: func(_ context.Context, o *order.Order) (*order.Order, error) {
			created = true
			return o, nil
		},
	}

	_, err := k.svc(t).PlaceOrder(context.Background(), k.input)
	require.ErrorIs(t, err, inbound.ErrNoValidAddress)
	assert.False(t, created, "no order row may be written before the address gate")
	assert.Nil(t, k.gotReq, "no workflow may start")
}

func TestOrderService_PlaceOrder_PharmacyFetchErrorIsGeneric(t *testing.T) {
	k := newOrderKit(t)
	k.pharm = fakePharmacyRepo{
		getByIDFn: func(context.Context, uuid.UUID) (*pharmacy.Pharmacy, error) { return nil, errBoom },
	}

	_, err := k.svc(t).PlaceOrder(context.Background(), k.input)
	require.ErrorIs(t, err, errBoom)
}

func TestOrderService_PlaceOrder_DuplicateMapsToAlreadyPlaced(t *testing.T) {
	// The partial unique index rejects a second active order for the offer.
	k := newOrderKit(t)
	k.ord.createFn = func(context.Context, *order.Order) (*order.Order, error) {
		return nil, outbound.ErrOrderExists
	}

	_, err := k.svc(t).PlaceOrder(context.Background(), k.input)
	require.ErrorIs(t, err, inbound.ErrOrderAlreadyPlaced)
}

func TestOrderService_PlaceOrder_CreateErrorIsGeneric(t *testing.T) {
	k := newOrderKit(t)
	k.ord.createFn = func(context.Context, *order.Order) (*order.Order, error) { return nil, errBoom }

	_, err := k.svc(t).PlaceOrder(context.Background(), k.input)
	require.ErrorIs(t, err, errBoom)
	assert.NotErrorIs(t, err, inbound.ErrOrderAlreadyPlaced)
}

func TestOrderService_PlaceOrder_StartFailureMarksOrderFailed(t *testing.T) {
	// A persisted order whose workflow never started is a dual-write orphan; the
	// service closes it by marking the row failed before returning.
	k := newOrderKit(t)
	k.starter = fakeOrderStarter{
		startFn: func(context.Context, order.PlacementRequest) error { return errBoom },
	}

	_, err := k.svc(t).PlaceOrder(context.Background(), k.input)
	require.ErrorIs(t, err, errBoom)
	require.NotNil(t, k.updated, "the order should have been updated to failed")
	assert.Equal(t, order.StatusFailed, k.updated.Status)
}

// --- GetOrderStatus ---------------------------------------------------------

func happyOrderByID(o *order.Order) fakeOrderRepo {
	return fakeOrderRepo{
		getByIDFn: func(context.Context, uuid.UUID) (*order.Order, error) { return o, nil },
	}
}

func happyQuerier(status string) fakeOrderStatusQuerier {
	return fakeOrderStatusQuerier{
		queryFn: func(context.Context, uuid.UUID) (string, error) { return status, nil },
	}
}

// statusKit wires an all-succeeding GetOrderStatus setup against consistent
// fixtures; a test swaps a single fake to drive one branch. User/pharmacy/starter
// are unused by GetOrderStatus, so they stay zero-value fakes.
type statusKit struct {
	userID  uuid.UUID
	orderID uuid.UUID
	ord     fakeOrderRepo
	offer   fakeOfferRepo
	rx      fakePrescriptionRepo
	querier fakeOrderStatusQuerier
}

func newStatusKit(t *testing.T) *statusKit {
	t.Helper()

	userID := uuid.New()
	orderRec, err := order.NewOrder(order.NewOrderParams{
		PrescriptionID: uuid.New(),
		OfferID:        uuid.New(),
		Qty:            30,
	})
	require.NoError(t, err)
	orderRec.ID = uuid.New()
	orderRec.Status = order.StatusShipped

	rx := buildRx(t, userID)
	quote := buildQuote(t, uuid.New(), "sku-1", 1299)
	offer := buildOffer(t, rx.ID, quote, time.Now().Add(time.Hour))

	return &statusKit{
		userID:  userID,
		orderID: orderRec.ID,
		ord:     happyOrderByID(orderRec),
		offer:   happyOffer(offer),
		rx:      happyRx(rx),
		querier: happyQuerier("in_transit"),
	}
}

func (k *statusKit) svc(t *testing.T) *app.OrderService {
	t.Helper()
	return newOrderSvc(t, k.offer, k.rx, fakeUserRepo{}, fakePharmacyRepo{}, k.ord, fakeOrderStarter{}, k.querier)
}

func TestOrderService_GetOrderStatus_HappyPath(t *testing.T) {
	k := newStatusKit(t)

	view, err := k.svc(t).GetOrderStatus(context.Background(), k.userID, k.orderID)
	require.NoError(t, err)

	assert.Equal(t, k.orderID, view.OrderID)
	assert.Equal(t, order.StatusShipped, view.Status)
	assert.Equal(t, "in_transit", view.ShippingStatus)
}

func TestOrderService_GetOrderStatus_OrderNotFound(t *testing.T) {
	k := newStatusKit(t)
	k.ord = fakeOrderRepo{
		getByIDFn: func(context.Context, uuid.UUID) (*order.Order, error) {
			return nil, outbound.ErrOrderNotFound
		},
	}

	_, err := k.svc(t).GetOrderStatus(context.Background(), k.userID, k.orderID)
	require.ErrorIs(t, err, inbound.ErrOrderNotFound)
}

func TestOrderService_GetOrderStatus_OrderFetchErrorIsGeneric(t *testing.T) {
	k := newStatusKit(t)
	k.ord = fakeOrderRepo{
		getByIDFn: func(context.Context, uuid.UUID) (*order.Order, error) { return nil, errBoom },
	}

	_, err := k.svc(t).GetOrderStatus(context.Background(), k.userID, k.orderID)
	require.ErrorIs(t, err, errBoom)
	assert.NotErrorIs(t, err, inbound.ErrOrderNotFound)
}

func TestOrderService_GetOrderStatus_OfferNotFoundHidesOrder(t *testing.T) {
	k := newStatusKit(t)
	k.offer = fakeOfferRepo{
		getByIDFn: func(context.Context, uuid.UUID) (*pharmacy.Offer, error) {
			return nil, outbound.ErrOfferNotFound
		},
	}

	_, err := k.svc(t).GetOrderStatus(context.Background(), k.userID, k.orderID)
	require.ErrorIs(t, err, inbound.ErrOrderNotFound)
}

func TestOrderService_GetOrderStatus_PrescriptionNotFoundHidesOrder(t *testing.T) {
	k := newStatusKit(t)
	k.rx = fakePrescriptionRepo{
		getByIDFn: func(context.Context, uuid.UUID) (*prescription.Prescription, error) {
			return nil, outbound.ErrPrescriptionNotFound
		},
	}

	_, err := k.svc(t).GetOrderStatus(context.Background(), k.userID, k.orderID)
	require.ErrorIs(t, err, inbound.ErrOrderNotFound)
}

func TestOrderService_GetOrderStatus_OwnershipMismatchHidesOrder(t *testing.T) {
	// An order whose prescription belongs to someone else collapses to not-found.
	k := newStatusKit(t)
	k.rx = happyRx(buildRx(t, uuid.New())) // owned by a different user

	_, err := k.svc(t).GetOrderStatus(context.Background(), k.userID, k.orderID)
	require.ErrorIs(t, err, inbound.ErrOrderNotFound)
}

func TestOrderService_GetOrderStatus_ClosedWorkflowFallsBackToOrderStatus(t *testing.T) {
	// A gone workflow (aged out / completed) yields no live detail: the coarse order
	// status is still returned, ShippingStatus empty, and it is not an error.
	k := newStatusKit(t)
	k.querier = fakeOrderStatusQuerier{
		queryFn: func(context.Context, uuid.UUID) (string, error) {
			return "", outbound.ErrOrderWorkflowNotFound
		},
	}

	view, err := k.svc(t).GetOrderStatus(context.Background(), k.userID, k.orderID)
	require.NoError(t, err)
	assert.Equal(t, order.StatusShipped, view.Status)
	assert.Empty(t, view.ShippingStatus)
}

func TestOrderService_GetOrderStatus_QueryErrorDegradesGracefully(t *testing.T) {
	// An unexpected query failure must not fail the endpoint; the order status still
	// answers, with no live shipping detail.
	k := newStatusKit(t)
	k.querier = fakeOrderStatusQuerier{
		queryFn: func(context.Context, uuid.UUID) (string, error) { return "", errBoom },
	}

	view, err := k.svc(t).GetOrderStatus(context.Background(), k.userID, k.orderID)
	require.NoError(t, err)
	assert.Equal(t, order.StatusShipped, view.Status)
	assert.Empty(t, view.ShippingStatus)
}

// --- ListOrders -------------------------------------------------------------

// listKit wires a two-prescription, two-order listing. Tests swap one fake to
// drive a single branch, and read gotRxIDs to see how the order query was scoped.
type listKit struct {
	userID    uuid.UUID
	rxOne     prescription.Prescription
	rxTwo     prescription.Prescription
	orders    []order.Order
	gotRxIDs  []uuid.UUID
	rxRepo    fakePrescriptionRepo
	orderRepo fakeOrderRepo
}

func newListKit(t *testing.T) *listKit {
	t.Helper()

	k := &listKit{userID: uuid.New()}
	k.rxOne = buildRx(t, k.userID)
	k.rxTwo = buildRx(t, k.userID)
	k.rxTwo.MedName = "lisinopril"

	paid, err := shared.NewMoneyFromCents(26400)
	require.NoError(t, err)

	k.orders = []order.Order{
		{
			ID:             uuid.New(),
			PrescriptionID: k.rxTwo.ID,
			OfferID:        uuid.New(),
			Status:         order.StatusDelivered,
			Qty:            30,
			PricePaid:      ptr.To(paid),
			PlacedAt:       time.Date(2026, 8, 2, 9, 0, 0, 0, time.UTC),
		},
		{
			ID:             uuid.New(),
			PrescriptionID: k.rxOne.ID,
			OfferID:        uuid.New(),
			Status:         order.StatusPlaced,
			Qty:            90,
			PricePaid:      nil,
			PlacedAt:       time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC),
		},
	}

	k.rxRepo = fakePrescriptionRepo{
		listFn: func(context.Context, uuid.UUID) ([]prescription.Prescription, error) {
			return []prescription.Prescription{k.rxOne, k.rxTwo}, nil
		},
	}
	k.orderRepo = fakeOrderRepo{
		listFn: func(_ context.Context, rxIDs []uuid.UUID) ([]order.Order, error) {
			k.gotRxIDs = rxIDs
			return k.orders, nil
		},
	}
	return k
}

func (k *listKit) svc(t *testing.T) *app.OrderService {
	t.Helper()
	return newOrderSvc(t, fakeOfferRepo{}, k.rxRepo, fakeUserRepo{}, fakePharmacyRepo{},
		k.orderRepo, fakeOrderStarter{}, fakeOrderStatusQuerier{})
}

func TestOrderService_ListOrders_ComposesNamesAndPreservesOrder(t *testing.T) {
	k := newListKit(t)

	views, err := k.svc(t).ListOrders(context.Background(), k.userID)
	require.NoError(t, err)
	require.Len(t, views, 2)

	// Each order's item name comes from its own prescription, not the first one.
	assert.Equal(t, "lisinopril", views[0].ItemName)
	assert.Equal(t, "atorvastatin", views[1].ItemName)

	// The repository decides ordering (newest first); the service must not reshuffle.
	assert.Equal(t, k.orders[0].ID, views[0].OrderID)
	assert.Equal(t, k.orders[1].ID, views[1].OrderID)

	assert.Equal(t, order.StatusDelivered, views[0].Status)
	assert.Equal(t, 30, views[0].Qty)
	assert.Equal(t, time.Date(2026, 8, 2, 9, 0, 0, 0, time.UTC), views[0].PlacedAt)
}

func TestOrderService_ListOrders_UncapturedOrderKeepsNilPrice(t *testing.T) {
	// PricePaid is nil until payment capture succeeds. It must survive as nil rather
	// than collapsing to a zero Money — $0.00 is a legitimate captured amount, so the
	// two cannot share a representation.
	k := newListKit(t)

	views, err := k.svc(t).ListOrders(context.Background(), k.userID)
	require.NoError(t, err)
	require.Len(t, views, 2)

	require.NotNil(t, views[0].PricePaid)
	assert.Equal(t, int64(26400), views[0].PricePaid.Cents())
	assert.Nil(t, views[1].PricePaid, "an uncaptured order must not report a price")
}

func TestOrderService_ListOrders_ScopesQueryToTheCallersPrescriptions(t *testing.T) {
	// Ownership here is enforced by construction: the only orders reachable are those
	// hanging off prescriptions fetched for this user, so the id set is the guarantee.
	k := newListKit(t)

	_, err := k.svc(t).ListOrders(context.Background(), k.userID)
	require.NoError(t, err)

	assert.ElementsMatch(t, []uuid.UUID{k.rxOne.ID, k.rxTwo.ID}, k.gotRxIDs)
}

func TestOrderService_ListOrders_NoPrescriptionsYieldsEmptyScope(t *testing.T) {
	// A user with no prescriptions can own no orders. The empty id set is what the
	// repository turns into "no rows" — it must never widen to every order.
	k := newListKit(t)
	k.rxRepo = fakePrescriptionRepo{
		listFn: func(context.Context, uuid.UUID) ([]prescription.Prescription, error) {
			return []prescription.Prescription{}, nil
		},
	}
	k.orderRepo = fakeOrderRepo{
		listFn: func(_ context.Context, rxIDs []uuid.UUID) ([]order.Order, error) {
			k.gotRxIDs = rxIDs
			return []order.Order{}, nil
		},
	}

	views, err := k.svc(t).ListOrders(context.Background(), k.userID)
	require.NoError(t, err)
	assert.Empty(t, k.gotRxIDs)
	assert.NotNil(t, views, "an empty result must marshal as [] rather than null")
	assert.Empty(t, views)
}

func TestOrderService_ListOrders_NoOrdersYieldsEmptySlice(t *testing.T) {
	k := newListKit(t)
	k.orderRepo = fakeOrderRepo{
		listFn: func(context.Context, []uuid.UUID) ([]order.Order, error) {
			return []order.Order{}, nil
		},
	}

	views, err := k.svc(t).ListOrders(context.Background(), k.userID)
	require.NoError(t, err)
	assert.NotNil(t, views, "an empty result must marshal as [] rather than null")
	assert.Empty(t, views)
}

func TestOrderService_ListOrders_RepositoryErrorsPropagate(t *testing.T) {
	// Neither repository failure is a client error — there is no not-found for a
	// collection, so both surface as a generic failure the handler turns into a 500.
	tests := map[string]func(k *listKit){
		"prescription repository fails": func(k *listKit) {
			k.rxRepo = fakePrescriptionRepo{
				listFn: func(context.Context, uuid.UUID) ([]prescription.Prescription, error) {
					return nil, errBoom
				},
			}
		},
		"order repository fails": func(k *listKit) {
			k.orderRepo = fakeOrderRepo{
				listFn: func(context.Context, []uuid.UUID) ([]order.Order, error) {
					return nil, errBoom
				},
			}
		},
	}

	for name, breakIt := range tests {
		t.Run(name, func(t *testing.T) {
			k := newListKit(t)
			breakIt(k)

			_, err := k.svc(t).ListOrders(context.Background(), k.userID)
			require.Error(t, err)
			assert.ErrorIs(t, err, errBoom)
		})
	}
}
