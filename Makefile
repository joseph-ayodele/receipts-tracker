.ONESHELL:
-include db/.env.db

# -----------------------------
# Runtime config (overridable)
# -----------------------------
DB_URL := postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(POSTGRES_DB)
GRPC_ADDR ?= :8080

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
	@echo "  -> Checking $(1)... "; \
	if command -v $(1) >/dev/null 2>&1 || which $(1) >/dev/null 2>&1 || where $(1) >/dev/null 2>&1; then \
		echo "Found."; \
	else \
		echo "NOT FOUND! Installing $(1)..."; \
		$(if $(2),$(2),echo "No installer defined for $(1)." && exit 1); \
		echo "Installation complete."; \
	fi
endef

# Platform-agnostic installation commands (returns command string)
define install_protoc
	if command -v apt >/dev/null 2>&1; then \
		sudo apt update && sudo apt install -y protobuf-compiler; \
	elif command -v yum >/dev/null 2>&1; then \
		sudo yum install -y protobuf-compiler; \
	elif command -v brew >/dev/null 2>&1; then \
		brew install protobuf; \
	elif command -v scoop >/dev/null 2>&1; then \
		scoop install protoc; \
	elif command -v choco >/dev/null 2>&1; then \
		choco install protoc; \
	else \
		echo "ERROR: No supported package manager found. Install protoc manually: https://grpc.io/docs/protoc-installation/"; \
		exit 1; \
	fi
endef

define install_tesseract
	if command -v apt >/dev/null 2>&1; then \
		sudo apt update && sudo apt install -y tesseract-ocr; \
	elif command -v yum >/dev/null 2>&1; then \
		sudo yum install -y tesseract; \
	elif command -v brew >/dev/null 2>&1; then \
		brew install tesseract; \
	elif command -v scoop >/dev/null 2>&1; then \
		scoop install tesseract; \
	elif command -v choco >/dev/null 2>&1; then \
		choco install tesseract; \
	else \
		echo "ERROR: No supported package manager found. Install tesseract manually."; \
		exit 1; \
	fi
endef

define install_poppler_utils
	if command -v apt >/dev/null 2>&1; then \
		sudo apt update && sudo apt install -y poppler-utils; \
	elif command -v yum >/dev/null 2>&1; then \
		sudo yum install -y poppler-utils; \
	elif command -v brew >/dev/null 2>&1; then \
		brew install poppler; \
	elif command -v scoop >/dev/null 2>&1; then \
		scoop install poppler; \
	elif command -v choco >/dev/null 2>&1; then \
		choco install poppler; \
	else \
		echo "ERROR: No supported package manager found. Install poppler-utils manually."; \
		exit 1; \
	fi
endef

define install_imagemagick
	if command -v apt >/dev/null 2>&1; then \
		sudo apt update && sudo apt install -y imagemagick; \
	elif command -v yum >/dev/null 2>&1; then \
		sudo yum install -y ImageMagick; \
	elif command -v brew >/dev/null 2>&1; then \
		brew install imagemagick; \
	elif command -v scoop >/dev/null 2>&1; then \
		scoop install imagemagick; \
	elif command -v choco >/dev/null 2>&1; then \
		choco install imagemagick; \
	else \
		echo "ERROR: No supported package manager found. Install imagemagick manually."; \
		exit 1; \
	fi
endef

# ==============================================================================
# DEPENDENCY CHECKS
# ==============================================================================
.PHONY: deps/go
deps/go: ## Verify that the Go compiler is installed
	@echo "Checking for Go compiler..."
	$(call check_prereq, go, @echo "ERROR: Go not found. Install Go from https://go.dev/doc/install" && exit 1)

.PHONY: deps/kubectl
deps/kubectl: ## Verify that kubectl is installed (for local dev env)
	@echo "Checking for kubectl..."
	$(call check_prereq, kubectl, @echo "ERROR: kubectl not found. Install from https://kubernetes.io/docs/tasks/tools/" && exit 1)

.PHONY: deps/tilt
deps/tilt: deps/kubectl ## Verify that Tilt is installed (for local dev env)
	@echo "Checking for Tilt..."
	$(call check_prereq, tilt, @echo "ERROR: tilt not found. Install from https://docs.tilt.dev/install.html" && exit 1)

.PHONY: deps/protoc
deps/protoc: ## Install pinned codegen tools (protoc plugins)
	@echo "Checking for code generation tools..."
	$(call check_prereq, protoc, $(install_protoc))

.PHONY: deps/ocr
deps/ocr: ## Checks for external binaries required for OCR processing.
	@echo "Checking for external binary dependencies for ocr..."
	$(call check_prereq, tesseract, $(install_tesseract))
	$(call check_prereq, pdftotext, $(install_poppler_utils))
	$(call check_prereq, pdftoppm, $(install_poppler_utils))
	$(call check_prereq, magick, $(install_imagemagick))
	@echo "All ocr binaries found."

.PHONY: deps
deps: deps/go deps/kubectl deps/tilt deps/protoc deps/ocr

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
