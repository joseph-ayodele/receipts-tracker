.ONESHELL:
-include db/.env.db

DB_URL := postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(POSTGRES_DB)

.PHONY: db/health
db/health:
	export DB_URL=$(DB_URL)
	go run ./cmd/dbhealth

.PHONY: srv/run
srv/run:
	export DB_URL=$(DB_URL)
	go run ./cmd/receiptsd

.PHONY: ent/generate
ent/generate:
	go run entgo.io/ent/cmd/ent generate --target gen/ent ./db/ent/schema

.PHONY: proto/generate
proto/generate:
	protoc -I . \
      --go_out=Mapi/receipts/v1/receipts.proto=proto/receipts/v1:./gen \
      --go-grpc_out=Mapi/receipts/v1/receipts.proto=proto/receipts/v1:./gen \
      api/receipts/v1/receipts.proto