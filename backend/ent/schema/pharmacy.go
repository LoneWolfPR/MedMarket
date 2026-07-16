package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// Pharmacy holds the schema definition for the Pharmacy entity.
type Pharmacy struct {
	ent.Schema
}

// Fields of the Pharmacy.
func (Pharmacy) Fields() []ent.Field {
	return []ent.Field{
		// Client-side default (uuid.New) mirrored with a DB-level default so
		// direct SQL inserts also get a generated id. See ent-db-defaults memory.
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Annotations(entsql.DefaultExpr("gen_random_uuid()")).
			Immutable(),
		// Stable business identifier (slug, e.g. "mock-pharmacy-a"). Explicitly
		// set by the app so config/adapters bind on it instead of the surrogate
		// PK; Unique so the seed's check-by-code is safe.
		field.String("code").
			Unique().
			NotEmpty(),
		field.String("name").
			NotEmpty(),
		field.String("contact_phone").
			NotEmpty(),
		// Regulatory identifiers — all three are legally unique registration
		// numbers, so each is Unique. The adapter maps any of these violations to
		// a single ErrPharmacyExists; field-specific reporting (via the pg
		// constraint name) can be added if a user-facing create/edit flow needs it.
		field.String("npi").
			Unique().
			NotEmpty(),
		field.String("dea").
			Unique().
			NotEmpty(),
		field.String("ncpdp").
			Unique().
			NotEmpty(),
		// Address value object, flattened into columns (mirrors the User schema).
		// street1/city/state/zip are required by Address.IsValid; street2 optional.
		field.String("address_street1").
			NotEmpty(),
		field.String("address_street2").
			Optional(),
		field.String("address_city").
			NotEmpty(),
		field.String("address_state").
			NotEmpty(),
		field.String("address_zip").
			NotEmpty(),
		// Timestamps. Client-side defaults mirrored with DB-level defaults so
		// direct SQL inserts populate them too. See ent-db-defaults memory.
		// Note: Postgres has no ON UPDATE, so updated_at's refresh on modify is
		// handled app-side by UpdateDefault; the DB default only covers inserts.
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

// Edges of the Pharmacy.
func (Pharmacy) Edges() []ent.Edge {
	return []ent.Edge{
		// Inverse side of Offer.pharmacy. Declared only so Ent emits the FK
		// constraint on offers.pharmacy_id; nothing traverses it.
		edge.To("offers", Offer.Type),
	}
}
