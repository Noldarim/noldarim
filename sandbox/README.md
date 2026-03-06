# Noldarim Dev Sandbox

Run the full Noldarim stack (server + Temporal + Docker-in-Docker) in an
isolated container using [Sysbox](https://github.com/nestybox/sysbox).

## Prerequisites

**Linux:**
```bash
# Install Sysbox
wget https://downloads.nestybox.com/sysbox/releases/v0.6.4/sysbox-ce_0.6.4-0.linux_amd64.deb
sudo dpkg -i sysbox-ce_0.6.4-0.linux_amd64.deb
```

**macOS:** Requires a Linux VM with Sysbox. Use Colima or Lima:
```bash
# Colima with Sysbox (if supported by your Colima version)
colima start --runtime sysbox --cpu 4 --memory 8

# Or Lima with manual Sysbox install
limactl start --name=noldarim template://docker
# Then SSH in and install Sysbox
```

## Usage

```bash
cd sandbox
docker compose up --build
```

The API server is available at `http://localhost:8080`.
Temporal UI is at `http://localhost:8233`.

## How It Works

The Sysbox runtime gives the sandbox container its own Linux user namespace
and Docker daemon. Pipeline agent containers are created inside the sandbox's
Docker — they are siblings, not nested. No privileged mode or socket mounting
is needed.

See `docs/plans/2026-03-06-sandbox-runtime-design.md` for the full design.
