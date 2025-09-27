package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"

	"github.com/google/uuid"
)

type Profile struct{ ent.Schema }

func (Profile) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "profiles"},
	}
}

func (Profile) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.String("name").NotEmpty(),
		field.String("default_currency").NotEmpty().MinLen(3).MaxLen(3).
			SchemaType(map[string]string{dialect.Postgres: "char(3)"}),
		field.Time("created_at").Default(time.Now),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (Profile) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("receipts", Receipt.Type),
		edge.To("files", ReceiptFile.Type),
		edge.To("jobs", ExtractJob.Type),
	}
}
