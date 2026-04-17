# DigiPulse Monitor (Go Service)

Ultra-fast, event-driven site monitoring service built in Go. This service handles the actual website reachability checks and reports results back to the Laravel backend.

## Technology Stack

- **Go 1.23**
- **Redis 7** (Event-driven communication)
- **Deployment**: Statically compiled binary in an Alpine container.

## Architecture

1.  **Job Acquisition**: Listens to a Redis channel for check tasks dispatched by the Laravel Scheduler.
2.  **Concurrency**: Uses Go routines to perform multiple checks (HTTP, SSL, Ping) simultaneously.
3.  **Reporting**: Sends results to the Backend API via authenticated webhooks.

## Deployment (CI/CD)

Deployments are automated via **GitHub Actions**.

### Workflow:
1.  **Build**: Compiles a static Linux binary (`CGO_ENABLED=0`).
2.  **Deploy**: The binary is uploaded to the server via SCP.
3.  **Environment**: A production `.env` file is generated on the server.
4.  **Runtime**: The `digipulse-monitor` container is restarted to execute the new binary.

### Required GitHub Secrets:

| Secret | Description |
|---|---|
| `SSH_KEY` | Private SSH key for the Hetzner server. |
| `REDIS_HOST` | Redis host (usually `digipulse-redis`). |
| `INTERNAL_MONITOR_KEY` | Shared secret for the Backend API. |

## Local Development

1.  Clone the repository.
2.  Start dependencies: `docker-compose up -d redis`.
3.  Run the application: `go run main.go`.

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `PORT` | 8080 | Local API port. |
| `REDIS_ADDR` | `localhost:6379` | Redis connection address. |
| `BACKEND_URL` | - | Webhook endpoint of the Laravel API. |
| `MONITOR_API_KEY` | - | Shared secret for verification. |

## Performance

- **Footprint**: < 10MB RAM usage.
- **Speed**: Optimized for sub-second DNS and HTTP resolution.