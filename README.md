# FinteraAPI - Go Backend

Go implementation of the FinteraAPI, migrated from Ruby on Rails.

## Quick Start

```bash
# Copy environment file
cp .env.example .env

# Edit .env with your database credentials
# The DATABASE_URL should point to your existing PostgreSQL database

# Download dependencies
make deps

# Run in development mode
make dev
```

## Project Structure

```
go-api/
├── cmd/api/           # Application entry point
├── internal/
│   ├── config/        # Configuration management
│   ├── database/      # Database connection
│   ├── models/        # Data models
│   ├── repository/    # Data access layer
│   ├── services/      # Business logic
│   ├── handlers/      # HTTP handlers
│   ├── middleware/    # HTTP middleware
│   ├── jobs/          # Background workers
│   ├── storage/       # File storage
│   └── utils/         # Utilities
├── pkg/               # Public packages
├── api/openapi/       # OpenAPI specs
├── Makefile          
├── Dockerfile
└── README.md
```

## Available Commands

```bash
make dev         # Run with hot reload (requires air)
make build       # Build binary
make run         # Build and run
make test        # Run tests
make lint        # Run linter
make swagger     # Generate API docs
make migrate-up  # Run migrations
```

## Key Features

- **Native Go concurrency** for background jobs (no external queue)
- **Local file storage** (no S3 dependency)
- **JWT authentication** compatible with Rails tokens
- **RBAC authorization** 
- **GORM ORM** for database operations
- **Gin framework** for HTTP routing

## Environment Variables

See `.env.example` for all configuration options.

## API Endpoints

All endpoints match the Rails API (see `/api/v1/...`).
