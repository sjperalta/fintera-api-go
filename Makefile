# Load .env file
ifneq (,$(wildcard ./.env))
    include .env
    export
endif

.PHONY: build run dev test clean migrate

# Build the application
build:
	go build -o bin/api ./cmd/api

# Run the application
run: build
	./bin/api

# Run in development mode with hot reload (requires air)
dev:
	@which air > /dev/null || go install github.com/air-verse/air@latest
	air

# Run tests
test:
	go test -v ./...

# Run tests with coverage
test-cover:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html

# Download dependencies
deps:
	go mod download
	go mod tidy

# Run database migrations (requires golang-migrate)
migrate-up:
	migrate -path internal/database/migrations -database "$(DATABASE_URL)" up

migrate-down:
	migrate -path internal/database/migrations -database "$(DATABASE_URL)" down 1

migrate-create:
	@read -p "Migration name: " name; \
	migrate create -ext sql -dir internal/database/migrations -seq $$name

# Run database seeds
seed:
	@if [ -z "$(DATABASE_URL)" ]; then echo "DATABASE_URL is not set"; exit 1; fi
	psql "$(DATABASE_URL)" -f internal/database/seeds/seeds.sql

# Generate Swagger docs (requires swag)
swagger:
	@which swag > /dev/null || go install github.com/swaggo/swag/cmd/swag@latest
	swag init -g cmd/api/main.go -o api/openapi

# Lint the code
lint:
	golangci-lint run

# Format code
fmt:
	go fmt ./...

# Check for security issues
security:
	gosec ./...

# Docker commands
docker-build:
	docker build -t fintera-api .

docker-run:
	docker run -p 8080:8080 --env-file .env fintera-api
