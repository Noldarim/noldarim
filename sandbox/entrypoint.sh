#!/bin/bash
# sandbox/entrypoint.sh
#
# Wait for the inner Docker daemon (started by systemd in the Sysbox image),
# then start the Noldarim server.

set -e

echo "Waiting for Docker daemon..."
timeout=30
while ! docker info >/dev/null 2>&1; do
    timeout=$((timeout - 1))
    if [ "$timeout" -le 0 ]; then
        echo "ERROR: Docker daemon did not start within 30 seconds"
        exit 1
    fi
    sleep 1
done
echo "Docker daemon is ready."

# Build the agent image inside the sandbox's Docker daemon
echo "Building agent image..."
docker build -t noldarim-agent -f - /usr/local/bin <<'EOF'
FROM ubuntu:22.04
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates git && rm -rf /var/lib/apt/lists/*
COPY noldarim-agent /app/agent
ENTRYPOINT ["/app/agent"]
EOF

echo "Starting Noldarim server..."
exec noldarim-server
