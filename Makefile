.PHONY: generate migrate migrate-down run test

# Подхватывает .env, который создаёт tripgoctl environment start.
load_env = set -a; [ -f .env ] && . ./.env; set +a;

generate:
	go tool oapi-codegen \
		-generate types,chi-server \
		-package api \
		-o api/api.gen.go \
		contracts/openapi/trip-service.openapi.yaml

migrate:
	$(load_env) go tool goose -dir migrations postgres "$$DATABASE_URL" up

migrate-down:
	$(load_env) go tool goose -dir migrations postgres "$$DATABASE_URL" down

run:
	$(load_env) go run ./cmd/trip-service

test:
	$(load_env) go test -race ./...
