#!/bin/bash
# build/agent-sysbox-startup.sh
#
# Startup script for the Sysbox agent container.
# Runs as a systemd service after Docker daemon is ready.
# Waits for Docker, then execs the agent binary.

set -e

# Wait for Docker daemon to be responsive
timeout=30
while ! docker info >/dev/null 2>&1; do
    timeout=$((timeout - 1))
    if [ "$timeout" -le 0 ]; then
        echo "ERROR: Docker daemon not responsive after 30s"
        exit 1
    fi
    sleep 1
done

exec /app/agent
