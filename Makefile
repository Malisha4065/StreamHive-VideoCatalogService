# Build and Development
.PHONY: build run test clean deps docker

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

BINARY_NAME=video-catalog-api
BINARY_PATH=./cmd/api

# Build the application
build:
	$(GOBUILD) -o $(BINARY_NAME) $(BINARY_PATH)

# Run the application
run:
	$(GOCMD) run $(BINARY_PATH)/main.go

# Run tests
test:
	$(GOTEST) -v ./...

# Run tests with coverage
test-coverage:
	$(GOTEST) -v -cover ./...

# Clean build artifacts
clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)

# Download dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy

# Build Docker image
docker-build:
	docker build -t streamhive/video-catalog-api:latest .

# Run with Docker Compose
docker-up:
	docker-compose up -d

# Stop Docker Compose
docker-down:
	docker-compose down

# View logs
logs:
	docker-compose logs -f video-catalog-api

# Database migration (when implemented)
migrate-up:
	@echo "Database auto-migration runs on startup"

# Lint code
lint:
	golangci-lint run

# Format code
fmt:
	$(GOCMD) fmt ./...

# Generate API documentation (when implemented)
docs:
	@echo "API documentation available in README.md"

# Development setup
dev-setup: deps docker-up
	@echo "Development environment ready!"
	@echo "API available at: http://localhost:8080"
	@echo "RabbitMQ Management: http://localhost:15672 (guest/guest)"
	@echo "PostgreSQL: localhost:5432 (postgres/postgres)"

# Help
help:
	@echo "Available commands:"
	@echo "  build        - Build the application"
	@echo "  run          - Run the application locally"
	@echo "  test         - Run tests"
	@echo "  test-coverage- Run tests with coverage"
	@echo "  clean        - Clean build artifacts"
	@echo "  deps         - Download dependencies"
	@echo "  docker-build - Build Docker image"
	@echo "  docker-up    - Start with Docker Compose"
	@echo "  docker-down  - Stop Docker Compose"
	@echo "  logs         - View application logs"
	@echo "  lint         - Lint code"
	@echo "  fmt          - Format code"
	@echo "  dev-setup    - Setup development environment"
	@echo "  help         - Show this help"
