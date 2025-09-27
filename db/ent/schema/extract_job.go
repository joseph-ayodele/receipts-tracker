package schema

import (
	"encoding/json"
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/joseph-ayodele/receipts-tracker/constants"
	"github.com/joseph-ayodele/receipts-tracker/utils"

	"github.com/google/uuid"
)

type ExtractJob struct{ ent.Schema }

func (ExtractJob) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "extract_job"},
	}
}

func (ExtractJob) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		// explicit FKs
		field.UUID("file_id", uuid.UUID{}),
		field.UUID("profile_id", uuid.UUID{}),
		field.UUID("receipt_id", uuid.UUID{}).Optional().Nillable(),
		field.String("format").NotEmpty().
			Validate(utils.EnumValidator(constants.FileTypes...)),
		field.Time("started_at").Default(time.Now),
		field.Time("finished_at").Optional().Nillable(),
		field.String("status").Optional().Nillable(),
		field.String("error_message").Optional().Nillable(),
		field.Float32("extraction_confidence").Optional().Nillable(),
		field.Bool("needs_review").Default(false),
		field.String("ocr_text").Optional().Nillable().
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.JSON("extracted_json", json.RawMessage{}).
			Optional(),
		field.String("model_name").Optional().Nillable(),
		field.JSON("model_params", json.RawMessage{}).
			Optional(),
	}
}

func (ExtractJob) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("file", ReceiptFile.Type).
			Ref("jobs").
			Field("file_id").
			Unique().
			Required(),
		edge.From("profile", Profile.Type).
			Ref("jobs").
			Field("profile_id").
			Unique().
			Required(),
		edge.From("receipt", Receipt.Type).
			Ref("jobs").
			Field("receipt_id").
			Unique(),
	}
}

func (ExtractJob) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("profile_id", "status", "started_at"),
		index.Fields("file_id"),
		index.Fields("receipt_id"),
	}
}
