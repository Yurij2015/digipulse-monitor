# Monitor Service

A Go-based monitoring service with Docker support.

## Project Structure

```
monitor/
├── cmd/
│   └── monitor/
│       └── main.go        # Application entry point
├── internal/              # Private application code
├── pkg/                   # Public library code
├── configs/               # Configuration files
├── main.go                # Root main.go (alternative entry point)
├── go.mod                 # Go module definition
├── Dockerfile             # Multi-stage Docker build
├── docker-compose.yml     # Docker Compose configuration
├── .gitignore             # Git ignore rules
└── .dockerignore          # Docker ignore rules
```

## Getting Started

### Prerequisites

- Go 1.22 or higher
- Docker and Docker Compose (optional)

### Running Locally

```bash
# Run directly with Go
go run main.go

# Or run from cmd directory
go run cmd/monitor/main.go
```

The service will start on port 8080 by default.

### Running with Docker

```bash
# Build and run with Docker Compose
docker-compose up --build

# Run in detached mode
docker-compose up -d --build

# Stop the service
docker-compose down
```

### Docker Development (hot reload)

For local containerized development with automatic restart on file changes, use the dev override:

```bash
# Start in foreground with hot reload
docker-compose -f docker-compose.yml -f docker-compose.dev.yml up --build

# Or use Makefile target
make compose-dev-up

# Stop dev stack
make compose-dev-down
```

This mode uses `Dockerfile.dev` + `.air.toml` and watches source files inside the mounted project directory.

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| PORT     | 8080    | Port the service listens on |

## API Endpoints

- `GET /` - Returns service status message
- `GET /health` - Health check endpoint (returns 200 OK)

## Development

### Building

```bash
# Build binary
go build -o main .

# Build with Docker
docker build -t monitor-service .
```

### Testing

```bash
# Run tests
go test ./...

# Run tests with coverage
go test -cover ./...
```

## License

[Add your license here]