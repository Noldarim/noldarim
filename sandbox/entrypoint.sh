#!/bin/bash
# sandbox/entrypoint.sh
#
# Runs as a systemd service after Docker daemon is ready.
# Builds the agent image, runs migrations, then starts the server.

set -e

cd /src/noldarim

# Wait for Docker socket to be fully responsive
timeout=30
while ! docker info >/dev/null 2>&1; do
    timeout=$((timeout - 1))
    if [ "$timeout" -le 0 ]; then
        echo "ERROR: Docker daemon not responsive after 30s"
        exit 1
    fi
    sleep 1
done

# Build the agent image inside the sandbox's Docker daemon
echo "Building agent image..."
docker build -t noldarim-agent -f - /usr/local/bin <<'EOF'
FROM ubuntu:22.04
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates git && rm -rf /var/lib/apt/lists/*
COPY noldarim-agent /app/agent
ENTRYPOINT ["/app/agent"]
EOF

echo "Running database migrations..."
noldarim-migrate

echo "Starting Noldarim server..."
exec noldarim-server
