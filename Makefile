.PHONY: build run clean test fmt vet

# Build the application
build:
	go build -o bin/server cmd/server/main.go

# Run the application
run:
	go run cmd/server/main.go

# Run the janitor sepperatly
janitor:
	go run cmd/janitor/main.go

# Run the simple main.go
run-simple:
	go run main.go

# Clean build artifacts
clean:
	rm -rf bin/

# Run tests
test:
	go test -v ./...

# Format code
fmt:
	go fmt ./...

# Vet code
vet:
	go vet ./...

# Run all checks
check: fmt vet test

# Install dependencies
deps:
	go mod tidy
	go mod download

# generate docs using swaggo
api-docs:
	swag init -d ./api/,./internal/handlers/,./internal/types/ -g ./router.go

# Development setup
dev-setup: deps fmt vet

# Development start caddy
dev-start-caddy:
	caddy start

# Development start caddy
dev-stop-caddy:
	caddy stop

# Development run
dev-run: dev-start-caddy run

# Build for production
build-prod:
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/server cmd/server/main.go