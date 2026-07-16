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

// Prescription holds the schema definition for the Prescription entity.
type Prescription struct {
	ent.Schema
}

// Fields of the Prescription.
func (Prescription) Fields() []ent.Field {
	return []ent.Field{
		// Client-side default (uuid.New) mirrored with a DB-level default so
		// direct SQL inserts also get a generated id. See ent-db-defaults memory.
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Annotations(entsql.DefaultExpr("gen_random_uuid()")).
			Immutable(),
		// Owning user. Bound to the edge below so the column carries a real FK
		// constraint; the domain still references the user by bare uuid and never
		// sees the edge. Indexed below for the per-user list.
		field.UUID("user_id", uuid.UUID{}).
			Immutable(),
		// MinIO object key ("<userID>/<uuid>.<ext>"); the blob lives in MinIO,
		// only the key is persisted. Required — upload precedes construction.
		field.String("document_key").
			NotEmpty().
			Immutable(),
		field.String("physician_name").
			NotEmpty(),
		field.String("med_name").
			NotEmpty(),
		// MedStrength value object, flattened into columns (mirrors the Address
		// flatten). Stored as the canonical strings the VO already holds; the
		// repo reconstructs via shared.NewMedStrength. See medstrength-string-storage.
		field.String("med_strength_value").
			NotEmpty(),
		field.String("med_strength_unit").
			NotEmpty(),
		field.Int("quantity").
			Positive(),
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

// Indexes of the Prescription.
func (Prescription) Indexes() []ent.Index {
	return []ent.Index{
		// Non-unique — a user has many prescriptions; speeds the per-user list.
		// Also the FK's referencing side, which Postgres does not index for us.
		index.Fields("user_id"),
	}
}

// Edges of the Prescription.
func (Prescription) Edges() []ent.Edge {
	return []ent.Edge{
		// Edges exist purely to generate the FK constraints — the adapter sets the
		// bare uuid fields and never traverses.
		edge.From("owner", User.Type).
			Ref("prescriptions").
			Field("user_id").
			Unique().
			Required().
			Immutable(),
		// Inverse side of Offer.prescription.
		edge.To("offers", Offer.Type),
		// Inverse side of Order.prescription.
		edge.To("orders", Order.Type),
	}
}
