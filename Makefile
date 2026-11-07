.PHONY: generate migrate migrate-down run test test-integration lint

# Подхватывает .env, который создаёт tripgoctl environment start.
load_env = set -a; [ -f .env ] && . ./.env; set +a;

generate:
	go tool oapi-codegen \
		-generate types,chi-server \
		-package api \
		-include-operation-ids createTrip,getTrip,finishTrip,health,ready \
		-o api/api.gen.go \
		contracts/openapi/trip-service.openapi.yaml
	go generate ./...

migrate:
	$(load_env) go tool goose -dir migrations postgres "$$DATABASE_URL" up

migrate-down:
	$(load_env) go tool goose -dir migrations postgres "$$DATABASE_URL" down

run:
	$(load_env) go run ./cmd/trip-service

test:
	$(load_env) go test -race ./...
	$(load_env) go test -race -coverprofile=cover.out ./internal/domain/... ./internal/usecase/...
	go tool cover -func=cover.out | grep total

test-integration:
	$(load_env) go test -race -tags=integration ./internal/repository/trip/

lint:
	$(load_env) golangci-lint run ./...
