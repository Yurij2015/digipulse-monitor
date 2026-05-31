# DigiPulse Monitor (Go Service)

Part of **DigiPulse** — a production SaaS for website and SSL monitoring built on an event-driven architecture: Laravel core dispatches check tasks via Redis queues; this Go worker processes them concurrently and pushes results back.

The service handles the actual reachability checks (HTTP, SSL, Ping) and is designed to stay out of the way: statically compiled, under 10 MB RAM, no runtime dependencies beyond Redis.

## Technology Stack

- **Go 1.23**
- **Redis 7** (event-driven task queue and result reporting)
- **Deployment**: static binary compiled with `CGO_ENABLED=0`, shipped in an Alpine container via GitHub Actions CI/CD

## Architecture

1. **Job Acquisition**: Reads check tasks from a Redis queue dispatched by the Laravel Scheduler.
2. **Concurrency**: Spawns a goroutine per check task — HTTP, SSL certificate, and Ping run in parallel per site.
3. **Reporting**: Pushes results to a Redis results queue consumed by Laravel (`app:consume-monitor-results`).
4. **Connectivity guard**: Probes outbound internet before each `BRPOP` cycle; backs off automatically when the network is unavailable instead of flooding error logs.

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
| `MONITOR_HEARTBEAT_KEY` | `go_monitor:last_heartbeat` | Redis key for worker liveness (use full key with Laravel `REDIS_PREFIX` in production, e.g. `laravel-database-go_monitor:last_heartbeat`). |
| `INTERNET_CHECK_ENABLED` | `true` | When `true`, the worker probes outbound internet before `BRPOP` and while the queue is idle; set `false` for intranet-only workers. |
| `INTERNET_PROBE_URL` | `https://www.cloudflare.com` | URL used for the connectivity probe (`GET`, short timeout). |
| `INTERNET_PROBE_TIMEOUT_SEC` | `5` | Timeout for a single probe request. |
| `INTERNET_OFFLINE_WAIT_SEC` | `10` | Sleep between probe retries when the network is down. |
| `REDIS_BRPOP_TIMEOUT_SEC` | `30` | `BRPOP` block duration when internet check is enabled (re-probes when the queue is empty). Ignored when `INTERNET_CHECK_ENABLED=false`. |

## Debugging

### Inspect env vars inside the container

`docker exec` spawns a new process that does not inherit variables set by the shell before `monitor_app` started. To inspect the environment of the main process (PID 1) directly:

```bash
docker exec digipulse-monitor cat /proc/1/environ | tr '\0' '\n' | grep -E "REDIS|HEARTBEAT"
```

- `/proc/1/environ` — Linux procfs file containing the env vars of PID 1 (`monitor_app`)
- `tr '\0' '\n'` — replaces null bytes (the delimiter between variables) with newlines
- `grep -E "REDIS|HEARTBEAT"` — filters for relevant variables

Further reading on Linux procfs: https://man7.org/linux/man-pages/man5/proc_pid_environ.5.html

## Performance

- **Footprint**: < 10 MB RSS in production.
- **Resolution**: Sub-second DNS and HTTP checks; goroutine-per-task concurrency keeps latency flat under load.
- **Binary**: statically compiled (`CGO_ENABLED=0`), no shared library dependencies — works in a bare Alpine image.