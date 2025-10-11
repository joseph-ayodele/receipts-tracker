.ONESHELL:
-include db/.env.db

# -----------------------------
# Runtime config (overridable)
# -----------------------------
DB_URL := postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(POSTGRES_DB)
GRPC_ADDR ?= :8080
# TODO: Update this path to your Tesseract data files location.
TESSDATA_PREFIX := C:/Users/joseph/scoop/apps/tesseract-languages/current

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
define check_prereq
	@echo "  -> Checking $(1)... "
	@if command -v $(1) >/dev/null 2>&1 || which $(1) >/dev/null 2>&1 || where $(1) >/dev/null 2>&1; then \
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

.PHONY: deps/kubectl
deps/kubectl: ## Verify that kubectl is installed (for local dev env)
	@echo "Checking for kubectl..."
	$(call check_prereq, kubectl, Install from https://kubernetes.io/docs/tasks/tools/)

.PHONY: deps/tilt
deps/tilt: deps/kubectl ## Verify that Tilt is installed (for local dev env)
	@echo "Checking for Tilt..."
	$(call check_prereq, tilt, Install from https://docs.tilt.dev/install.html)

.PHONY: deps/protoc
deps/protoc: ## Install pinned codegen tools (protoc plugins)
	@echo "Checking for code generation tools..."
	$(call check_prereq, protoc, Install from https://grpc.io/docs/protoc-installation/)
	@echo "Installing code generation tools..."
#	go install google.golang.org/protobuf/cmd/protoc-gen-go@$(PROTOC_GEN_GO_VERSION)
#	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@$(PROTOC_GEN_GO_GRPC_VERSION)

.PHONY: deps/ocr-tools
deps/ocr-tools: ## Checks for external binaries required for OCR processing.
	@echo "Checking for external binary dependencies..."
	$(call check_prereq, tesseract, Use 'brew/scoop install tesseract'.)
	$(call check_prereq, pdftotext, Poppler utilities not found. Use 'brew/scoop install poppler'.)
	$(call check_prereq, pdftoppm, Poppler utilities not found. Use 'brew/scoop install poppler'.)
	$(call check_prereq, magick, ImageMagick not found. Use 'brew/scoop install imagemagick'.)
	@echo "All external binaries found."

.PHONY: deps/ocr
deps/ocr: deps/ocr-tools
	@echo "Using TESSDATA_PREFIX=$(TESSDATA_PREFIX)"

.PHONY: deps
deps: deps/go deps/kubectl deps/protoc

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
proto/generate: deps/protoc ## Generate protobuf + gRPC stubs into ./gen
	protoc -I . \
	  --go_out=Mapi/receipts/v1/common.proto=proto/receipts/v1,Mapi/receipts/v1/profiles.proto=proto/receipts/v1,Mapi/receipts/v1/receipts.proto=proto/receipts/v1,Mapi/receipts/v1/ingest.proto=proto/receipts/v1,Mapi/receipts/v1/export.proto=proto/receipts/v1:./gen \
	  --go-grpc_out=Mapi/receipts/v1/common.proto=proto/receipts/v1,Mapi/receipts/v1/profiles.proto=proto/receipts/v1,Mapi/receipts/v1/receipts.proto=proto/receipts/v1,Mapi/receipts/v1/ingest.proto=proto/receipts/v1,Mapi/receipts/v1/export.proto=proto/receipts/v1:./gen \
	  api/receipts/v1/*.proto

.PHONY: generate
generate: ent/generate proto/generate

# -----------------------------
# Build / Run
# -----------------------------
.PHONY: build/server
build/server: ## Build the gRPC server binary
	go build -o bin/receiptsd ./cmd/receiptsd

.PHONY: srv/run
srv/run: deps ## Run the gRPC server (uses DB_URL and GRPC_ADDR)
	export DB_URL=$(DB_URL)
	export GRPC_ADDR=$(GRPC_ADDR)
	# If you configure TESSDATA_PREFIX here, it will override system env.
ifneq ($(strip $(TESSDATA_PREFIX)),)
	export TESSDATA_PREFIX=$(TESSDATA_PREFIX)
endif
	go run ./cmd/receiptsd

.PHONY: srv/run-ocr
srv/run-ocr: deps ## Run the gRPC server (uses DB_URL and GRPC_ADDR)
	export DB_URL=$(DB_URL)
	export GRPC_ADDR=$(GRPC_ADDR)
	# If you configure TESSDATA_PREFIX here, it will override system env.
ifneq ($(strip $(TESSDATA_PREFIX)),)
	export TESSDATA_PREFIX=$(TESSDATA_PREFIX)
endif
	go run ./cmd/runocr

# -----------------------------
# Misc
# -----------------------------
.PHONY: help
help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nTargets:\n"} /^[a-zA-Z0-9_\/-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)
	@echo ""
	@echo "Current config:"
	@echo "  DB_URL          = $(DB_URL)"
	@echo "  GRPC_ADDR       = $(GRPC_ADDR)"
	@echo "  TESSDATA_PREFIX = $(TESSDATA_PREFIX)"
	@echo "Tool versions:"
	@echo "  protoc-gen-go        = $(PROTOC_GEN_GO_VERSION)"
	@echo "  protoc-gen-go-grpc   = $(PROTOC_GEN_GO_GRPC_VERSION)"
	@echo "  ent (go run)         = $(ENT_VERSION)"
