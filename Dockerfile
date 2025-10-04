# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum, then download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code (entire project for internal/gen imports)
COPY . .

# Build the receipts-tracker binary
RUN go build -o receipts-tracker ./cmd/receipts-tracker

# Final stage
FROM alpine:latest

WORKDIR /app

# Install OCR tools and dependencies
RUN apk add --no-cache tesseract-ocr tesseract-ocr-data-eng poppler-utils imagemagick

# Copy the binary from the builder stage
COPY --from=builder /app/receipts-tracker ./receipts-tracker

# Expose the port (update if your app uses a different port)
EXPOSE 8080

# Set TESSDATA_PREFIX (can be overridden in Kubernetes)
ENV TESSDATA_PREFIX=/usr/share/tessdata

ENTRYPOINT ["./receipts-tracker"]
