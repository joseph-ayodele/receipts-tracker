package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// Category maps to the existing public.categories table.
type Category struct {
	ent.Schema
}

func (Category) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "categories"},
	}
}

func (Category) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable().
			StorageKey("id"),
		field.String("name").
			NotEmpty().
			Unique().
			SchemaType(map[string]string{dialect.Postgres: "text"}),
	}
}

func (Category) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("receipts", Receipt.Type),
	}
}
