package schema

import (
	"errors"
	"regexp"
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"

	"github.com/google/uuid"
)

var reLast4 = regexp.MustCompile(`^[0-9]{4}$`)

type Receipt struct{ ent.Schema }

func (Receipt) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "receipts"},
	}
}

func (Receipt) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable().
			StorageKey("id"),
		field.UUID("profile_id", uuid.UUID{}),
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
		// FK fields created by edges; no need to duplicate.
		field.String("payment_method").Optional().Nillable(),
		field.String("payment_last4").Optional().Nillable().
			Validate(func(s string) error {
				if s == "" || reLast4.MatchString(s) {
					return nil
				}
				return reLast4Err
			}),
		field.String("description").Optional().Nillable(),
		field.Time("created_at").Default(time.Now),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

var reLast4Err = errors.New("invalid last 4 digits")

func (Receipt) Edges() []ent.Edge {
	return []ent.Edge{
		// MANY receipts -> ONE profile (FK: receipts.profile_id)
		edge.From("profile", Profile.Type).
			Ref("receipts").
			Field("profile_id").
			Required().
			Unique(),
		// OPTIONAL: MANY receipts -> ONE category (FK: receipts.category_id)
		edge.From("category", Category.Type).
			Ref("receipts").
			Field("category_id").
			Unique(),
		// ONE receipt -> MANY files
		edge.To("files", ReceiptFile.Type),
		// ONE receipt -> MANY jobs
		edge.To("jobs", ExtractJob.Type),
	}
}
