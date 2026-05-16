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
| `MONITOR_HEARTBEAT_KEY` | `go_monitor:last_heartbeat` | Redis key for worker liveness (use full key with Laravel `REDIS_PREFIX` in production, e.g. `laravel-database-go_monitor:last_heartbeat`). |
| `INTERNET_CHECK_ENABLED` | `true` | When `true`, the worker probes outbound internet before `BRPOP` and while the queue is idle; set `false` for intranet-only workers. |
| `INTERNET_PROBE_URL` | `https://www.cloudflare.com` | URL used for the connectivity probe (`GET`, short timeout). |
| `INTERNET_PROBE_TIMEOUT_SEC` | `5` | Timeout for a single probe request. |
| `INTERNET_OFFLINE_WAIT_SEC` | `10` | Sleep between probe retries when the network is down. |
| `REDIS_BRPOP_TIMEOUT_SEC` | `30` | `BRPOP` block duration when internet check is enabled (re-probes when the queue is empty). Ignored when `INTERNET_CHECK_ENABLED=false`. |

## Removing sensitive data from git history

If a file with credentials or server config (e.g. `deployment-config.json`) was accidentally committed, use [`git filter-repo`](https://github.com/newren/git-filter-repo) to erase it from the entire history:

```bash
# Install (macOS)
brew install git-filter-repo

# Rewrite history — removes the file from every commit on every branch
git filter-repo --path deployment-config.json --invert-paths --force

# git filter-repo removes 'origin' as a safety measure; add it back
git remote add origin <repo-url>

# Force-push the rewritten history
git push --force origin main
```

**How it works:** `git filter-repo` replays every commit and drops any tree entry matching `--path`. `--invert-paths` means "keep everything *except* this path." The result is a new linear history where the file never existed. All commit SHAs change, so every collaborator must re-clone (`git clone`) — their existing local copies are incompatible with the new remote history.

> **Warning:** this is a destructive, irreversible operation. GitHub caches may retain the old objects for up to 90 days; if the leak is critical, contact GitHub Support to purge the cache immediately.

Official docs: https://htmlpreview.github.io/?https://github.com/newren/git-filter-repo/blob/docs/html/git-filter-repo.html

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

- **Footprint**: < 10MB RAM usage.
- **Speed**: Optimized for sub-second DNS and HTTP resolution.