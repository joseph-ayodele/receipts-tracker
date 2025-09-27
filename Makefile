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
# Define a single, reusable macro for checking prerequisites
# $(1) = Binary name (e.g., protoc)
# $(2) = Specific error/installation message
define check_prereq
	@echo -n "  -> Checking $(1)... "
	if command -v $(1) >/dev/null 2>&1; then \
		echo "Found."; \
	else \
		echo "NOT FOUND!"; \
		echo "ERROR: $(1) not found. $(2)"; \
		exit 1; \
	fi
endef

# ==============================================================================
# DEPENDENCY CHECKS (Platform-Agnostic)
# ==============================================================================
.PHONY: deps/go
deps/go: ## Verify that the Go compiler is installed
	@echo "Checking for Go compiler..."
	$(call check_prereq, go, Install Go from https://go.dev/doc/install)

.PHONY: deps/tools
deps/tools: deps/go ## Install pinned codegen tools (protoc plugins)
	$(call check_prereq, protoc, Install from https://grpc.io/docs/protoc-installation/)

	@echo "Installing code generation tools..."
	go install google.golang.org/protobuf/cmd/protoc-gen-go@$(PROTOC_GEN_GO_VERSION)
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@$(PROTOC_GEN_GO_GRPC_VERSION)
	@echo "Tools installed:"
	@echo "  protoc-gen-go        @ $(PROTOC_GEN_GO_VERSION)"
	@echo "  protoc-gen-go-grpc   @ $(PROTOC_GEN_GO_GRPC_VERSION)"

.PHONY: deps/bin
deps/bin: ## Checks for external binaries required for OCR processing.
	@echo "Checking for external binary dependencies..."
	# Check Tesseract
	$(call check_prereq, tesseract, Use 'brew/scoop install tesseract' or similar.)

	# Check Poppler utilities
	$(call check_prereq, pdftotext, Poppler utilities not found. Use 'brew/scoop install poppler' or similar.)
	$(call check_prereq, pdftoppm, Poppler utilities not found. Use 'brew/scoop install poppler' or similar.)

	@echo "All external binaries found."

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

.PHONY: srv/dev
srv/dev: ## Run the gRPC server with live reload style (same as srv/run, alias for dev)
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
