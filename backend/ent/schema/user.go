// Package schema defines the Ent entity schemas.
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// User holds the schema definition for the User entity.
type User struct {
	ent.Schema
}

// Fields of the User.
func (User) Fields() []ent.Field {
	return []ent.Field{
		// Client-side default (uuid.New) mirrored with a DB-level default so
		// direct SQL inserts also get a generated id. See ent-db-defaults memory.
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Annotations(entsql.DefaultExpr("gen_random_uuid()")).
			Immutable(),
		field.String("email").
			Unique().
			NotEmpty(),
		field.String("password_hash").
			NotEmpty().
			Sensitive(),
		field.String("first_name").
			NotEmpty(),
		field.String("last_name").
			NotEmpty(),
		field.String("phone").
			Optional(),
		// Address value object, flattened into columns (see review note on the
		// flatten-vs-JSON choice).
		field.String("address_street1").
			Optional(),
		field.String("address_street2").
			Optional(),
		field.String("address_city").
			Optional(),
		field.String("address_state").
			Optional(),
		field.String("address_zip").
			Optional(),
	}
}

// Edges of the User.
func (User) Edges() []ent.Edge {
	return nil
}
