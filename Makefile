APP_NAME := monitor-service
VERSION := 1.0.0

.PHONY: all
all: build

.PHONY: build
build:
	@echo "Building $(APP_NAME) $(VERSION)..."
	go build -o bin/$(APP_NAME) main.go

.PHONY: run
run:
	@echo "Running $(APP_NAME)..."
	go run main.go

.PHONY: test
test:
	@echo "Running tests..."
	go test -v ./...

.PHONY: test-coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

.PHONY: swagger
swagger:
	@echo "Generating Swagger documentation..."
	swag init -g main.go -o api/swagger

.PHONY: docker-build
docker-build:
	@echo "Building Docker image..."
	docker build -t $(APP_NAME):$(VERSION) .

.PHONY: docker-run
docker-run:
	@echo "Running Docker container..."
	docker run -p 8080:8080 $(APP_NAME):$(VERSION)

.PHONY: compose-up
compose-up:
	@echo "Starting services with Docker Compose..."
	docker-compose up --build

.PHONY: compose-up-detached
compose-up-detached:
	@echo "Starting services with Docker Compose (detached)..."
	docker-compose up -d --build

.PHONY: compose-down
compose-down:
	@echo "Stopping services..."
	docker-compose down

.PHONY: clean
clean:
	@echo "Cleaning up..."
	rm -rf bin/
	rm -f coverage.out coverage.html

.PHONY: deps
deps:
	@echo "Downloading dependencies..."
	go mod download
	go install github.com/swaggo/swag/cmd/swag@latest

.PHONY: tidy
tidy:
	@echo "Tidying modules..."
	go mod tidy

.PHONY: fmt
fmt:
	@echo "Formatting code..."
	go fmt ./...

.PHONY: lint
lint:
	@echo "Running linter..."
	go vet ./...

.PHONY: check
check: fmt lint test

.PHONY: help
help:
	@echo "Available commands:"
	@echo "  build           Build the application"
	@echo "  run             Run the application locally"
	@echo "  test            Run tests"
	@echo "  test-coverage   Run tests with coverage report"
	@echo "  swagger         Generate Swagger documentation"
	@echo "  docker-build    Build Docker image"
	@echo "  docker-run      Run Docker container"
	@echo "  compose-up      Start services with Docker Compose"
	@echo "  compose-down    Stop services"
	@echo "  clean           Clean build artifacts"
	@echo "  deps            Download dependencies"
	@echo "  tidy            Tidy Go modules"
	@echo "  fmt             Format code"
	@echo "  lint            Run linter"
	@echo "  check           Run fmt, lint and tests"