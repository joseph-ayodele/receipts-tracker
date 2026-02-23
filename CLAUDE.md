# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

### Build & Run
```bash
go build -o bin/receiptsd ./cmd/receipts-tracker   # Build the server binary
go run ./cmd/receipts-tracker                       # Run directly (requires env vars)
go run ./cmd/receipts-tracker -inmem                # Run with in-memory SQLite (no DB_URL needed)
```

> Note: The Makefile's `build/server` and `srv/run` targets reference `./cmd/receiptsd` (a typo — the actual path is `./cmd/receipts-tracker`).

### Code Generation
```bash
make generate               # Run both ent + proto generation
make ent/generate           # Regenerate Ent ORM code → gen/ent/
make proto/generate         # Regenerate gRPC stubs → gen/proto/
```

**Never edit files under `gen/`** — they are fully generated.

### Debug / Manual Testing CLIs
```bash
go run ./cmd/runocr         # Run OCR pipeline on a file
go run ./cmd/llm            # Run LLM extraction standalone
go run ./cmd/receipt-batch  # Batch-process receipts
go run ./cmd/dbhealth       # Check DB connectivity
```

### Database
```bash
make db/health              # Ping DB and run ent query (requires db/.env.db)
```

## Architecture

### Layer Boundaries

The codebase has a strict 4-tier layered architecture. Each layer has a single direction of dependency:

```
gRPC Request
     ↓
internal/server/        ← Proto ↔ service type translation, gRPC status codes
     ↓
internal/services/      ← Business logic, validation, queue dispatch
     ↓
internal/repository/    ← DB access, returns internal/entity/ types
     ↓
internal/core/          ← OCR + LLM pipeline (processor.go, async/, ocr/, llm/)
```

- **`internal/server/`**: Thin adapters. No business logic. Translates between `*v1.FooRequest` and service-layer types, assigns gRPC status codes.
- **`internal/services/`**: All business logic and validation. Four services: `ingest`, `receipt`, `profile`, `export`.
- **`internal/repository/`**: All DB access via Ent. Repository interfaces are defined here; implementations use `*ent.Client`. Returns `internal/entity/` domain types (not raw Ent nodes).
- **`internal/core/`**: Pure processing engine. `processor.go` orchestrates OCR → LLM → DB upsert. `async/processor_queue.go` manages a worker pool (6 workers, 512-item channel, 3min per-job timeout).

### Processing Pipeline

1. **Ingest**: `IngestFile`/`IngestDirectory` on `IngestionServer` → `ingest.Service` → `FSIngestor` computes SHA256, deduplicates on `(profile_id, content_hash)`, creates `ReceiptFile` row, enqueues `async.Job`.
2. **OCR Worker** (`core.Processor.ProcessFile`): Picks job from channel → runs OCR (PDF: `pdftotext` or `pdftoppm`+tesseract; image: tesseract; HEIC: magick→PNG first) → writes `ocr_text` + confidence to `extract_jobs`, advances status `RUNNING → OCR_OK`.
3. **LLM Parse** (`runLLMParse`): Reads `ocr_text`; if OCR confidence < threshold, attaches raw file as vision input to OpenAI call → extracts fields → upserts `receipts` row, advances job to `LLM_OK` or `FAILED`.

### Key Files

| File | Purpose |
|------|---------|
| `cmd/receipts-tracker/main.go` | Server entrypoint: wires all layers together, starts gRPC |
| `internal/common/config.go` | All env-var config loading + DB initialization helpers |
| `internal/core/processor.go` | OCR→LLM orchestration |
| `internal/core/async/processor_queue.go` | Worker pool |
| `internal/core/ocr/` | PDF and image OCR; confidence scoring |
| `internal/core/llm/` | OpenAI client, prompt building, validation, sanitization |
| `db/ent/schema/` | Ent entity definitions (source of truth for schema) |
| `api/receipts/v1/*.proto` | Proto service definitions |
| `constants/` | Category definitions, file type → format mapping, job status enums |
| `internal/entity/` | Domain types returned by repositories |

### Data Flow for Schema Changes

Schema changes must follow this order:
1. Edit `db/ent/schema/*.go`
2. Run `make ent/generate` → updates `gen/ent/`
3. Update `internal/repository/` implementations if needed
4. For proto changes: edit `api/receipts/v1/*.proto`, then `make proto/generate`

### Configuration

Required env vars: `DB_URL`, `OPENAI_API_KEY`
Optional: `GRPC_ADDR` (default `:8080`), `OPENAI_MODEL` (default `gpt-4o-mini`), `OPENAI_TEMPERATURE`, `OPENAI_TIMEOUT`, `HEIC_CONVERTER` (default `magick`), `TESSDATA_PREFIX`, `ARTIFACT_CACHE_DIR` (default `./tmp`), DB pool tuning vars.

Use `-inmem` flag to bypass `DB_URL` requirement and run with SQLite in-memory.

### Conventions

- Repository interfaces live in `internal/repository/*.go` alongside their implementations.
- gRPC status codes are assigned **only** in `internal/server/`; services return `status.Error(...)` directly since they are called from gRPC handlers.
- `constants.Canonicalize()` maps raw LLM category strings to canonical `Category` constants; unknown categories set `needs_review = true` on the job.
- Logging uses `log/slog` with structured key-value pairs. Log level is `INFO` in production; `DEBUG` lines exist for OCR/LLM details.
- The `extract_jobs.status` column progresses: `QUEUED → RUNNING → OCR_OK → LLM_OK | FAILED`.
