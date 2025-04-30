.PHONY: build test run clean

# Variables
BINARY_NAME=monlift
GO=go
DOCKER=docker
DOCKER_COMPOSE=docker-compose

# Build the application
build:
	$(GO) build -o $(BINARY_NAME) main.go

# Run tests
test:
	$(GO) test -v ./...

# Run tests with coverage
test-coverage:
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out

# Run the application
run:
	$(GO) run main.go

# Clean build artifacts
clean:
	$(GO) clean
	rm -f $(BINARY_NAME)
	rm -f coverage.out

# Start MongoDB container
mongo-up:
	$(DOCKER_COMPOSE) up -d

# Stop MongoDB container
mongo-down:
	$(DOCKER_COMPOSE) down

# Create a new migration
create-migration:
	@if [ -z "$(name)" ]; then \
		echo "Please provide a migration name: make create-migration name=your_migration_name"; \
		exit 1; \
	fi
	$(GO) run main.go create $(name)

# Run migrations up
migrate-up:
	$(GO) run main.go up

# Run migrations down
migrate-down:
	$(GO) run main.go down

# Help command
help:
	@echo "Available commands:"
	@echo "  make build              - Build the application"
	@echo "  make test              - Run tests"
	@echo "  make test-coverage     - Run tests with coverage"
	@echo "  make run               - Run the application"
	@echo "  make clean             - Clean build artifacts"
	@echo "  make mongo-up          - Start MongoDB container"
	@echo "  make mongo-down        - Stop MongoDB container"
	@echo "  make create-migration  - Create a new migration (requires name parameter)"
	@echo "  make migrate-up        - Run migrations up"
	@echo "  make migrate-down      - Run migrations down"
	@echo "  make help              - Show this help message"
