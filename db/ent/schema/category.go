package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
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
		field.Int("id").Immutable(),
		field.String("name").NotEmpty().Unique().
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Enum("category_type").
			Values("DIRECT", "INDIRECT").
			Default("DIRECT").
			StorageKey("category_type"),
	}
}

func (Category) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("receipts", Receipt.Type),
	}
}
