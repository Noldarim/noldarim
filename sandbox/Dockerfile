# sandbox/Dockerfile
#
# Noldarim sandbox image for Sysbox runtime.
# Includes systemd + Docker daemon + Noldarim binaries.

FROM nestybox/ubuntu-jammy-systemd-docker:latest

# Install build dependencies
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      ca-certificates curl git make sqlite3 && \
    rm -rf /var/lib/apt/lists/*

# Install Go (use a specific version for reproducibility)
RUN curl -fsSL https://go.dev/dl/go1.23.6.linux-amd64.tar.gz | tar -C /usr/local -xz
ENV PATH="/usr/local/go/bin:${PATH}"

# Copy source and build
COPY . /src/noldarim
WORKDIR /src/noldarim
RUN go build -o /usr/local/bin/noldarim-server ./cmd/server && \
    go build -o /usr/local/bin/noldarim-agent ./cmd/agent

# Copy entrypoint
COPY sandbox/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
