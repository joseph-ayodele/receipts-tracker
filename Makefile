.ONESHELL:
-include db/.env.db

.PHONY: db/health
db/health:
	DB_URL="postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(POSTGRES_DB)"
	export DB_URL
	go run ./cmd/dbhealth

.PHONY: srv/run proto/gen

srv/run:
	go run ./cmd/receiptsd

.PHONY: proto/gen
ent/generate:
	go run entgo.io/ent/cmd/ent generate --target gen/ent ./db/ent/schema

proto/generate:
	protoc -I api --go_out=. --go-grpc_out=. api/receipts/v1/receipts.proto