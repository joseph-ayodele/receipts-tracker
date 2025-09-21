.ONESHELL:
-include db/.env.db

.PHONY: db/health
db/health:
	DB_URL="postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(POSTGRES_DB)"
	export DB_URL
	go run ./cmd/dbhealth
