APP_NAME := monitor-service

.PHONY: all
all: build

.PHONY: build
build:
	@echo "Building $(APP_NAME)..."
	go build -o bin/$(APP_NAME) main.go

.PHONY: test
test:
	@echo "Running tests..."
	go test -v ./...

.PHONY: swagger
swagger:
	@echo "Generating Swagger documentation..."
	swag init -g main.go -o api/swagger

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
	@echo "Running linter (go vet)..."
	go vet ./...

.PHONY: clean
clean:
	@echo "Cleaning up..."
	rm -rf bin/

.PHONY: check
check: fmt lint tidy build test