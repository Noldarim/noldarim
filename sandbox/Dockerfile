# sandbox/Dockerfile
#
# Noldarim sandbox image for Sysbox runtime.
# Uses systemd base image (arm64 support). Systemd manages Docker daemon.
# Noldarim starts via a oneshot service after Docker is ready.

FROM nestybox/ubuntu-jammy-systemd-docker:latest

# Install build dependencies
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      ca-certificates curl git make && \
    rm -rf /var/lib/apt/lists/*

# Install Go (use a specific version for reproducibility)
ARG GO_VERSION=1.26
ARG TARGETARCH
RUN curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${TARGETARCH}.tar.gz" | tar -C /usr/local -xz
ENV PATH="/usr/local/go/bin:${PATH}"

# Copy source and build
COPY . /src/noldarim
WORKDIR /src/noldarim
RUN go build -o /usr/local/bin/noldarim-server ./cmd/server && \
    go build -o /usr/local/bin/noldarim-agent ./cmd/agent && \
    go build -o /usr/local/bin/noldarim-migrate ./cmd/migrate

# Overwrite project root config with sandbox config (viper searches CWD first)
COPY sandbox/config.yaml /src/noldarim/config.yaml
COPY sandbox/config.yaml /etc/noldarim/config.yaml

# Copy startup script
COPY sandbox/entrypoint.sh /usr/local/bin/noldarim-startup.sh
RUN chmod +x /usr/local/bin/noldarim-startup.sh

# Create a simple systemd service that runs after Docker is ready
RUN printf '[Unit]\n\
Description=Noldarim Server\n\
After=docker.service\n\
Requires=docker.service\n\
\n\
[Service]\n\
Type=simple\n\
WorkingDirectory=/src/noldarim\n\
ExecStart=/usr/local/bin/noldarim-startup.sh\n\
Restart=on-failure\n\
RestartSec=5\n\
StartLimitBurst=3\n\
StandardOutput=journal+console\n\
StandardError=journal+console\n\
\n\
[Install]\n\
WantedBy=multi-user.target\n' > /etc/systemd/system/noldarim.service && \
    systemctl enable noldarim.service

# systemd is PID 1 (from base image)
