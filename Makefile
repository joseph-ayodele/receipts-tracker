.ONESHELL:
-include db/.env.db

.PHONY: db/health
db/health:
	DB_URL="postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(POSTGRES_DB)"
	export DB_URL
	go run ./cmd/dbhealth

srv/run:
	go run ./cmd/receiptsd

ent/generate:
	go run entgo.io/ent/cmd/ent generate --target gen/ent ./db/ent/schema

proto/generate:
	protoc -I . \
      --go_out=Mapi/receipts/v1/receipts.proto=proto/receipts/v1:./gen \
      --go-grpc_out=Mapi/receipts/v1/receipts.proto=proto/receipts/v1:./gen \
      api/receipts/v1/receipts.proto