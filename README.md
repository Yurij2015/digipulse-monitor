# DigiPulse Monitor (Go Service)

Ultra-fast, event-driven site monitoring service built in Go. This service handles the actual website reachability checks and reports results back to the Laravel backend via Redis queues.

## Technology Stack

- **Go 1.23**
- **Redis 7** (Event-driven communication)
- **Deployment**: Statically compiled binary in an Alpine container.

## Architecture

1.  **Job Acquisition**: Reads check tasks from a Redis queue dispatched by the Laravel Scheduler.
2.  **Concurrency**: Uses Go routines to perform multiple checks (HTTP, SSL, Ping) simultaneously.
3.  **Reporting**: Pushes check results to a Redis results queue consumed by Laravel (`app:consume-monitor-results`).

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

## Local Development

1.  Clone the repository.
2.  Start dependencies: `docker-compose up -d redis`.
3.  Run the application: `go run main.go`.

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `PORT` | 8080 | Local API port. |
| `REDIS_ADDR` | `localhost:6379` | Redis connection address. |
| `MONITOR_REDIS_CHANNEL` | `monitoring:tasks` | Redis queue with pending check tasks. |
| `MONITOR_RESULTS_CHANNEL` | `monitoring:results` | Redis queue where processed check results are pushed. |
| `INTERNET_CHECK_ENABLED` | `true` | When `true`, the worker probes outbound internet before `BRPOP` and while the queue is idle; set `false` for intranet-only workers. |
| `INTERNET_PROBE_URL` | `https://www.cloudflare.com` | URL used for the connectivity probe (`GET`, short timeout). |
| `INTERNET_PROBE_TIMEOUT_SEC` | `5` | Timeout for a single probe request. |
| `INTERNET_OFFLINE_WAIT_SEC` | `10` | Sleep between probe retries when the network is down. |
| `REDIS_BRPOP_TIMEOUT_SEC` | `30` | `BRPOP` block duration when internet check is enabled (re-probes when the queue is empty). Ignored when `INTERNET_CHECK_ENABLED=false`. |

## Performance

- **Footprint**: < 10MB RAM usage.
- **Speed**: Optimized for sub-second DNS and HTTP resolution.