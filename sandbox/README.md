# Noldarim Dev Sandbox

Run the full Noldarim stack (server + Temporal + Docker-in-Docker) in an
isolated container using [Sysbox](https://github.com/nestybox/sysbox).

## Prerequisites

**Linux (amd64):**
```bash
wget https://downloads.nestybox.com/sysbox/releases/v0.6.4/sysbox-ce_0.6.4-0.linux_amd64.deb
sudo dpkg -i sysbox-ce_0.6.4-0.linux_amd64.deb
```

**Linux (arm64):**
```bash
wget https://downloads.nestybox.com/sysbox/releases/v0.6.4/sysbox-ce_0.6.4-0.linux_arm64.deb
sudo dpkg -i sysbox-ce_0.6.4-0.linux_arm64.deb
```

**macOS (Apple Silicon / Intel):**

Requires a Linux VM with Sysbox. Use Colima (recommended):

```bash
# 1. Install Colima (stop Docker Desktop first)
brew install colima docker docker-compose

# 2. Start a VM (native arch, no emulation)
colima start --cpu 4 --memory 8 --disk 60

# 3. SSH in and install Sysbox
colima ssh
# Inside VM — detect arch and install matching package:
ARCH=$(dpkg --print-architecture)  # amd64 or arm64
wget https://downloads.nestybox.com/sysbox/releases/v0.6.4/sysbox-ce_0.6.4-0.linux_${ARCH}.deb
sudo dpkg -i sysbox-ce_0.6.4-0.linux_${ARCH}.deb
exit

# 4. Restart Colima to pick up the new runtime
colima stop && colima start --cpu 4 --memory 8 --disk 60
```

## Usage

```bash
cd sandbox
docker compose up --build
```

The API server is available at `http://localhost:8080`.
Temporal UI is at `http://localhost:8233`.

To tear down (including volumes):
```bash
docker compose down -v
```

## What's Inside

| Service | Purpose |
|---------|---------|
| `postgres` | Temporal persistence (PostgreSQL 16) |
| `temporal` | Workflow engine (auto-setup with schema migration) |
| `noldarim` | Sysbox container: systemd → Docker daemon → agent image build → DB migration → API server |

## How It Works

The Sysbox runtime gives the sandbox container its own Linux user namespace
and Docker daemon. Pipeline agent containers are created inside the sandbox's
Docker — they are siblings, not nested. No privileged mode or socket mounting
is needed.

The `nestybox/ubuntu-jammy-systemd-docker` base image runs systemd as PID 1,
which manages the inner Docker daemon. The Noldarim server runs as a systemd
service that starts after Docker is ready.

## Code Delivery

In the sandbox, binaries are built from source during `docker compose build`.
The agent image is built inside the sandbox's Docker daemon on first start
(see `entrypoint.sh`).

For local development (without the sandbox), project code is delivered to
agent containers via git worktree bind mounts. See `CLAUDE.md` in the project
root for the full code delivery matrix.

See `docs/plans/2026-03-06-sandbox-runtime-design.md` for the full design.
