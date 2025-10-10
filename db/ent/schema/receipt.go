package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/google/uuid"
)

type Receipt struct{ ent.Schema }

func (Receipt) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "receipts"},
	}
}

func (Receipt) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.UUID("profile_id", uuid.UUID{}),
		field.UUID("file_id", uuid.UUID{}).Optional().Nillable(),

		field.String("merchant_name").NotEmpty(),
		field.Time("tx_date").
			SchemaType(map[string]string{dialect.Postgres: "date"}).
			Immutable(),
		field.Float("subtotal").
			Optional().Nillable().
			SchemaType(map[string]string{dialect.Postgres: "numeric(12,2)"}),
		field.Float("tax").
			Optional().Nillable().
			SchemaType(map[string]string{dialect.Postgres: "numeric(12,2)"}),
		field.Float("total").
			SchemaType(map[string]string{dialect.Postgres: "numeric(12,2)"}),
		field.String("currency_code").NotEmpty().MinLen(3).MaxLen(3).
			SchemaType(map[string]string{dialect.Postgres: "char(3)"}),

		field.String("category_name").NotEmpty(),
		field.String("description"),
		field.String("file_path").Optional().Nillable(),
		field.Bool("is_current").Default(true),
		field.Time("created_at").Default(time.Now),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (Receipt) Edges() []ent.Edge {
	return []ent.Edge{
		// MANY receipts -> ONE profile (FK: receipts.profile_id)
		edge.From("profile", Profile.Type).
			Ref("receipts").
			Field("profile_id").
			Required().
			Unique(),
		// ONE receipt -> MANY files
		edge.To("files", ReceiptFile.Type),
		// ONE receipt -> MANY jobs
		edge.To("jobs", ExtractJob.Type),
	}
}

func (Receipt) Indexes() []ent.Index {
	return []ent.Index{
		// Performance indexes (unique constraints must be added via SQL migration)
		index.Fields("profile_id", "tx_date"),
		index.Fields("profile_id", "category_name"),
		index.Fields("profile_id", "merchant_name"),
	}
}
