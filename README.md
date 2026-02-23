# receipts-tracker

A self-hosted receipt processing pipeline that ingests receipt files (PDFs, images, HEIC), extracts structured financial data using OCR and an LLM, and exports to Excel for tax reporting.

## What it does

1. **Ingests** a directory of receipt files — deduplicates by content hash so re-runs are safe
2. **Extracts** data from each file via one of two modes:
   - **OCR → LLM**: Tesseract extracts text, OpenAI parses fields from the text
   - **Vision-direct**: skips OCR entirely, sends the file directly to the LLM as a vision input (more accurate for image receipts and scanned PDFs)
3. **Normalizes** extracted fields — resolves gift card offsets, reconciles totals, canonicalizes expense categories
4. **Exports** to a `.xlsx` spreadsheet formatted for tax deduction reporting (transaction date, expense category, item, amount, notes, file path)

## Modes

### Batch CLI (primary)
Processes a directory end-to-end and writes an Excel file. No server required.

```bash
OPENAI_API_KEY=... OPENAI_MODEL=gpt-4o OPENAI_TEMPERATURE=0 \
go run ./cmd/receipt-batch \
  --dir "/path/to/receipts" \
  --vision-direct
```

Output defaults to `receipts-<timestamp>.xlsx` in the parent of `--dir`.

### gRPC Server
Long-running server with a full gRPC API for ingestion, querying, and export. Backed by PostgreSQL in production, SQLite in-memory for local use.

```bash
go run ./cmd/receipts-tracker -inmem   # local / no DB required
```

## Supported file types

| Format | OCR method | Vision-direct |
|--------|-----------|---------------|
| PDF | `pdftotext` (text), `pdftoppm` + Tesseract (scanned) | PDF pages rasterized via `pdftoppm`, up to 5 pages |
| JPEG / PNG | Tesseract | Attached directly |
| HEIC | `magick` → PNG → Tesseract | `magick` → PNG → attached |

## Expense categories

`Office Supplies` · `Office Equipment` · `Home Office` · `Software Subscription` · `Cell Phone Service` · `Internet` · `Meals` · `Shipping Expenses` · `Professional Development` · `Travel Expenses` · `Other`

## Requirements

- Go 1.24+
- `tesseract` — OCR engine
- `pdftotext` + `pdftoppm` — PDF processing (Poppler)
- `magick` — HEIC/HEIF conversion (ImageMagick)
- OpenAI API key (`OPENAI_API_KEY`)
- PostgreSQL (production) or `-inmem` flag (SQLite, no setup)

## Infrastructure

Kubernetes manifests and a `Tiltfile` are included for local cluster development (`k8s/`, `Tiltfile`). The Docker image bundles all OCR dependencies.

```bash
tilt up   # starts receipts-tracker + postgres in local k8s
```

## Configuration

| Env var | Default | Description |
|---------|---------|-------------|
| `OPENAI_API_KEY` | — | Required |
| `OPENAI_MODEL` | `gpt-4o-mini` | Recommend `gpt-4o` for accuracy |
| `OPENAI_TEMPERATURE` | `0.0` | |
| `DB_URL` | — | PostgreSQL DSN (not needed with `-inmem`) |
| `GRPC_ADDR` | `:8080` | gRPC listen address |
| `HEIC_CONVERTER` | `magick` | `magick` \| `sips` \| `heif-convert` |
| `ARTIFACT_CACHE_DIR` | `./tmp` | Cached HEIC→PNG conversions |
| `TESSDATA_PREFIX` | — | Path to Tesseract language data |
