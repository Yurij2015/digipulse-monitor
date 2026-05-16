#!/usr/bin/env bash
set -euo pipefail

# Post-deploy on the app server (restart Go monitor container after binary + .env update).
if docker ps -a --format '{{.Names}}' | grep -q 'digipulse-monitor'; then
  docker restart digipulse-monitor
  sleep 3
  if curl -sf --max-time 5 http://127.0.0.1:8080/health >/dev/null; then
    echo 'Monitor health check OK'
  else
    echo 'Monitor restarted but /health did not respond yet (may need a few seconds)'
  fi
else
  echo 'Container digipulse-monitor not found. Run terraform apply first.'
  exit 1
fi
