package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Offer holds the schema definition for the Offer entity.
type Offer struct {
	ent.Schema
}

// Fields of the Offer.
func (Offer) Fields() []ent.Field {
	return []ent.Field{
		// Client-side default (uuid.New) mirrored with a DB-level default so
		// direct SQL inserts also get a generated id. See ent-db-defaults memory.
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Annotations(entsql.DefaultExpr("gen_random_uuid()")).
			Immutable(),
		// The prescription this offer was quoted for. Bound to the edge below so
		// the column carries a real FK constraint; the domain still references the
		// prescription by bare uuid and never sees the edge.
		field.UUID("prescription_id", uuid.UUID{}).
			Immutable(),
		// PriceQuote value object, flattened into columns (mirrors the Address and
		// MedStrength flattens). The repo reconstructs it via pharmacy.NewPriceQuote.
		//
		// NonNegative, not Positive: a $0.00 quote is legitimate (discounts / full
		// insurance coverage) and shared.Money already rejects negatives.
		field.Int64("price_cents").
			NonNegative().
			Immutable(),
		// The quoting pharmacy. Also edge-bound (see below) for the FK.
		field.UUID("pharmacy_id", uuid.UUID{}).
			Immutable(),
		// The pharmacy's own item handle — A's sku, B's code.
		field.String("pharmacy_item_id").
			NotEmpty().
			Immutable(),
		// The instant this offer stops being honored. Set by the app service as
		// now+OFFER_TTL; the order flow compares against it before placing.
		field.Time("expires_at").
			Immutable(),
		// Timestamps. Client-side defaults mirrored with DB-level defaults so
		// direct SQL inserts populate them too. See ent-db-defaults memory.
		// Every business field above is Immutable (an offer is a price pinned to a
		// moment and is never rewritten), so updated_at can only ever equal
		// created_at today; it is kept for symmetry with the other tables.
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

// Indexes of the Offer.
func (Offer) Indexes() []ent.Index {
	return []ent.Index{
		// Postgres does not index the referencing side of a FK (unlike InnoDB), so
		// both FK columns are indexed explicitly: they are the join keys for any
		// out-of-app querying, and an unindexed child column makes parent deletes
		// scan the whole table.
		index.Fields("prescription_id"),
		index.Fields("pharmacy_id"),
	}
}

// Edges of the Offer.
func (Offer) Edges() []ent.Edge {
	return []ent.Edge{
		// Edges exist purely to generate the FK constraints — the adapter sets the
		// bare uuid fields above and never traverses. Bound via Field() so the
		// column stays explicit rather than Ent inventing its own.
		edge.From("prescription", Prescription.Type).
			Ref("offers").
			Field("prescription_id").
			Unique().
			Required().
			Immutable(),
		edge.From("pharmacy", Pharmacy.Type).
			Ref("offers").
			Field("pharmacy_id").
			Unique().
			Required().
			Immutable(),
	}
}
