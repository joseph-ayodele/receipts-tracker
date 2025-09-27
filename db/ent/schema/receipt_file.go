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

type ReceiptFile struct {
	ent.Schema
}

func (ReceiptFile) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "receipt_files"},
	}
}

func (ReceiptFile) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable().
			StorageKey("id"),
		// explicit FKs so we can define a composite unique index
		field.UUID("profile_id", uuid.UUID{}),
		field.String("source_path").NotEmpty(),
		field.Bytes("content_hash").NotEmpty().
			SchemaType(map[string]string{dialect.Postgres: "bytea"}),
		field.String("filename").NotEmpty(),
		field.String("file_ext").NotEmpty(),
		field.Int("file_size").NonNegative(),
		field.Time("uploaded_at").Default(time.Now),
	}
}

func (ReceiptFile) Edges() []ent.Edge {
	return []ent.Edge{
		// MANY files -> ONE profile
		edge.From("profile", Profile.Type).
			Ref("files").
			Field("profile_id").
			Required().
			Unique(),
		// ONE file -> MANY jobs
		edge.To("jobs", ExtractJob.Type),
	}
}

func (ReceiptFile) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("profile_id", "content_hash").Unique(),
		index.Fields("profile_id", "uploaded_at"),
	}
}
