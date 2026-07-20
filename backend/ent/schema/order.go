package schema

import (
	"strings"
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"

	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/order"
)

// Order holds the schema definition for the Order entity.
type Order struct {
	ent.Schema
}

// statusCheck renders the status CHECK from the domain's own list, so the legal
// set has exactly one definition. Adding a status there reaches the constraint
// through a regenerate + migration rather than a second list drifting here.
func statusCheck() string {
	quoted := make([]string, len(order.AcceptedStatuses))
	for i, s := range order.AcceptedStatuses {
		quoted[i] = "'" + s + "'"
	}
	return "status IN (" + strings.Join(quoted, ", ") + ")"
}

// Annotations of the Order.
func (Order) Annotations() []schema.Annotation {
	return []schema.Annotation{
		// The domain no longer validates status (NewOrder always starts at placed,
		// and the saga advances it by assignment), so the database is what keeps a
		// typo out of the column — the same reasoning as the foreign keys.
		entsql.Annotation{
			Checks: map[string]string{
				"order_status_valid": statusCheck(),
			},
		},
	}
}

// Fields of the Order.
func (Order) Fields() []ent.Field {
	return []ent.Field{
		// Client-side default (uuid.New) mirrored with a DB-level default so
		// direct SQL inserts also get a generated id. See ent-db-defaults memory.
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Annotations(entsql.DefaultExpr("gen_random_uuid()")).
			Immutable(),
		// The prescription being ordered, and the offer whose price was accepted.
		// Both are edge-bound below for the FK; the domain keeps bare uuids.
		field.UUID("prescription_id", uuid.UUID{}).
			Immutable(),
		field.UUID("offer_id", uuid.UUID{}).
			Immutable(),
		// Order vocabulary only — shipping's own states map into this and are never
		// stored raw. Mutable: the saga advances it. Constrained by the CHECK above.
		field.String("status"),
		// The pharmacy's own handles, returned by PlaceOrder. Empty until the saga
		// has actually reached the pharmacy; "" is an unambiguous "not set" for a
		// string, so these need no NULL.
		field.String("pharmacy_order_id").
			Optional(),
		field.String("tracking_id").
			Optional(),
		// Copied from the prescription at order time (the prescription is the source
		// of truth), so the order records what was actually bought.
		field.Int("quantity").
			Positive().
			Immutable(),
		// What the payment processor actually captured — NULL until it does, which is
		// why this is Nillable rather than defaulting to 0: $0.00 is a legitimate
		// captured amount (full insurance coverage), so zero cannot mean "unpaid".
		// The authorized amount is deliberately absent: it is derivable as
		// offer price x quantity, and only the uncomputable fact is stored.
		field.Int64("price_paid_cents").
			NonNegative().
			Optional().
			Nillable(),
		// Timestamps. Client-side defaults mirrored with DB-level defaults so
		// direct SQL inserts populate them too. See ent-db-defaults memory.
		// Postgres has no ON UPDATE, so updated_at's refresh is app-side via
		// UpdateDefault; the DB default only covers inserts.
		field.Time("created_at").
			Default(time.Now).
			Immutable().
			Annotations(entsql.DefaultExpr("CURRENT_TIMESTAMP")),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now).
			Annotations(entsql.DefaultExpr("CURRENT_TIMESTAMP")),
	}
}

// Indexes of the Order.
func (Order) Indexes() []ent.Index {
	return []ent.Index{
		// Postgres does not index the referencing side of a FK (unlike InnoDB), so
		// both FK columns are indexed explicitly. tracking_id is deliberately not
		// indexed yet — the shipping webhook that looks orders up by it lands at
		// step 7, and an index without a query is premature.
		index.Fields("prescription_id"),
		index.Fields("offer_id"),
		// One live order per offer: a partial unique index that excludes the
		// terminal-non-success statuses, so a failed/canceled attempt can be
		// re-ordered but an active one cannot be duplicated. This is the only
		// reliable guard against a concurrent double-submit (an app-level
		// check-then-insert has a TOCTOU race). The predicate is raw SQL and
		// won't move if the constants change — keep it in sync with
		// order.StatusFailed and order.StatusCanceled.
		index.Fields("offer_id").
			Unique().
			StorageKey("order_active_offer_uniq").
			Annotations(entsql.IndexWhere("status NOT IN ('failed', 'canceled')")),
	}
}

// Edges of the Order.
func (Order) Edges() []ent.Edge {
	return []ent.Edge{
		// Edges exist purely to generate the FK constraints — the adapter sets the
		// bare uuid fields above and never traverses.
		edge.From("prescription", Prescription.Type).
			Ref("orders").
			Field("prescription_id").
			Unique().
			Required().
			Immutable(),
		edge.From("offer", Offer.Type).
			Ref("orders").
			Field("offer_id").
			Unique().
			Required().
			Immutable(),
	}
}
