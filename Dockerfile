# Build stage
FROM golang:alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main ./cmd/api

# Final stage
FROM alpine:latest

# Install dependencies: make, curl (for migrate), ca-certificates, tzdata, and libraries for wkhtmltopdf
RUN apk --no-cache add \
    ca-certificates \
    tzdata \
    make \
    curl \
    libc6-compat \
    libstdc++ \
    libx11 \
    libxrender \
    libxext \
    libssl3 \
    fontconfig \
    freetype \
    ttf-dejavu \
    ttf-droid \
    ttf-freefont \
    ttf-liberation

# Copy wkhtmltopdf from specialized image
COPY --from=surnet/alpine-wkhtmltopdf:3.19.0-0.12.6-full /bin/wkhtmltopdf /bin/wkhtmltopdf

# Install golang-migrate
RUN curl -L https://github.com/golang-migrate/migrate/releases/download/v4.17.0/migrate.linux-amd64.tar.gz | tar xvz && \
    mv migrate /usr/local/bin/

WORKDIR /root/

# Copy binary from builder
COPY --from=builder /app/main .

# Copy migration files, seed files, and Makefile for automated migrations
COPY internal/database/migrations ./internal/database/migrations
COPY internal/database/seeds ./internal/database/seeds
COPY Makefile .

# Create storage directory
RUN mkdir -p /root/storage

# Expose port
EXPOSE 8080

# Run migrations and then start the application
CMD ["sh", "-c", "make migrate-up && make seed && ./main"]
