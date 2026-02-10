# Load .env file if it exists, but don't override existing environment variables
# Using -include ignores the file if it doesn't exist
-include .env

# Ensure DATABASE_URL is exported and respects environment overrides
# If DATABASE_URL was in .env, 'include' set it. If it was already in env, 
# 'include' might have overridden it depending on 'make' version.
# To be safe, we can use the following pattern:
export DATABASE_URL ?= $(DATABASE_URL)

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

# Drop daabase and run seed and migrations
drop-db-seed-migrate:
	psql "$(subst fintera_api_development,postgres,$(DATABASE_URL))" -c "DROP DATABASE IF EXISTS fintera_api_development;"
	psql "$(subst fintera_api_development,postgres,$(DATABASE_URL))" -c "CREATE DATABASE fintera_api_development;"
	migrate -path internal/database/migrations -database "$(DATABASE_URL)" up
	psql "$(DATABASE_URL)" -f internal/database/seeds/seeds.sql

# Drop daabase and run seed and migrations and force drop database
drop-db-seed-migrate-force:
	psql "$(subst fintera_api_development,postgres,$(DATABASE_URL))" -c "SELECT pg_terminate_backend(pg_stat_activity.pid) FROM pg_stat_activity WHERE pg_stat_activity.datname = 'fintera_api_development' AND pid <> pg_backend_pid();"
	psql "$(subst fintera_api_development,postgres,$(DATABASE_URL))" -c "DROP DATABASE IF EXISTS fintera_api_development;"
	psql "$(subst fintera_api_development,postgres,$(DATABASE_URL))" -c "CREATE DATABASE fintera_api_development;"
	migrate -path internal/database/migrations -database "$(DATABASE_URL)" up
	psql "$(DATABASE_URL)" -f internal/database/seeds/seeds.sql

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
