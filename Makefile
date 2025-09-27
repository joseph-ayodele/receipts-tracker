.ONESHELL:
-include db/.env.db

# -----------------------------
# Runtime config (overridable)
# -----------------------------
DB_URL := postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(POSTGRES_DB)
GRPC_ADDR ?= :50051

# -----------------------------
# Tooling dependency matrix
# -----------------------------
# You can bump these centrally.
PROTOC_GEN_GO_VERSION        ?= v1.34.2
PROTOC_GEN_GO_GRPC_VERSION   ?= v1.5.1
ENT_VERSION                  ?= latest         # ent is used via `go run` below; keep for visibility.

# -----------------------------
# Helpers
# -----------------------------
define need_protoc
	@command -v protoc >/dev/null 2>&1 || { \
	  echo "ERROR: protoc not found. Install from https://grpc.io/docs/protoc-installation/"; \
	  exit 1; \
	}
endef

.PHONY: deps/tools
deps/tools: ## Install pinned codegen tools (protoc plugins)
	$(call need_protoc)
	go install google.golang.org/protobuf/cmd/protoc-gen-go@$(PROTOC_GEN_GO_VERSION)
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@$(PROTOC_GEN_GO_GRPC_VERSION)
	@echo "Tools installed:"
	@echo "  protoc-gen-go        @ $(PROTOC_GEN_GO_VERSION)"
	@echo "  protoc-gen-go-grpc   @ $(PROTOC_GEN_GO_GRPC_VERSION)"

# -----------------------------
# Database utilities
# -----------------------------
.PHONY: db/health
db/health: ## Run DB health check (pgx ping + ent query)
	export DB_URL=$(DB_URL)
	go run ./cmd/dbhealth

# -----------------------------
# Code generation
# -----------------------------
.PHONY: ent/generate
ent/generate: ## Generate ent code (to gen/ent)
	go run entgo.io/ent/cmd/ent generate --target gen/ent ./db/ent/schema

.PHONY: proto/generate
proto/generate: deps/tools ## Generate protobuf + gRPC stubs into ./gen
	protoc -I . \
	  --go_out=Mapi/receipts/v1/common.proto=proto/receipts/v1,Mapi/receipts/v1/profiles.proto=proto/receipts/v1,Mapi/receipts/v1/receipts.proto=proto/receipts/v1,Mapi/receipts/v1/ingest.proto=proto/receipts/v1,Mapi/receipts/v1/export.proto=proto/receipts/v1:./gen \
	  --go-grpc_out=Mapi/receipts/v1/common.proto=proto/receipts/v1,Mapi/receipts/v1/profiles.proto=proto/receipts/v1,Mapi/receipts/v1/receipts.proto=proto/receipts/v1,Mapi/receipts/v1/ingest.proto=proto/receipts/v1,Mapi/receipts/v1/export.proto=proto/receipts/v1:./gen \
	  api/receipts/v1/*.proto

# -----------------------------
# Build / Run
# -----------------------------
.PHONY: build/server
build/server: ## Build the gRPC server binary
	go build -o bin/receiptsd ./cmd/receiptsd

.PHONY: srv/run
srv/run: ## Run the gRPC server (uses DB_URL and GRPC_ADDR)
	export DB_URL=$(DB_URL)
	export GRPC_ADDR=$(GRPC_ADDR)
	go run ./cmd/receiptsd

# -----------------------------
# Misc
# -----------------------------
.PHONY: help
help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nTargets:\n"} /^[a-zA-Z0-9_\/-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)
	@echo ""
	@echo "Current config:"
	@echo "  DB_URL     = $(DB_URL)"
	@echo "  GRPC_ADDR  = $(GRPC_ADDR)"
	@echo "Tool versions:"
	@echo "  protoc-gen-go        = $(PROTOC_GEN_GO_VERSION)"
	@echo "  protoc-gen-go-grpc   = $(PROTOC_GEN_GO_GRPC_VERSION)"
	@echo "  ent (go run)         = $(ENT_VERSION)"
