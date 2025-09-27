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

# Option A: local protoc (brew/choco/winget)
.PHONY: proto/gen
proto/gen:
	protoc -I . --go_out=paths=source_relative:. --go-grpc_out=paths=source_relative:. ./gen/proto/receipts/v1/receipts.proto


# Option B: Dockerized protoc (no local install required)
proto/gen-docker:
	docker run --rm -v $(PWD):/work -w /work ghcr.io/namely/docker-protoc:1.57_1 \
	  -I . --go_out=. --go-grpc_out=. api/receipts/v1/receipts.proto