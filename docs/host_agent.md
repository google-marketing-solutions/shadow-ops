# Host Agent Architecture (`host-agent/`)

The Host Agent is a lightweight Node.js Express service listening on port `:8081`. It manages the local Docker Engine daemon via `/var/run/docker.sock`, orchestrating isolated Workspace Agent containers for browser automation sessions.

---

## Core Responsibilities

### 1. Docker Daemon Integration (`index.js`)
- Uses `dockerode` to interact directly with the host's Docker socket.
- Manages container lifecycles, health inspections, and automatic garbage collection of orphaned or exited containers.

### 2. Workspace Container Provisioning (`POST /containers`)
- **Image**: Spawns containers from `shadowops-workspace-agent:dev`.
- **Networking**: Configures containers with `HostConfig: { NetworkMode: 'host' }` for low-latency loopback streaming.
- **Port Assignment**: Passes dynamic `AGENT_PORT` environment variables to each container.
- **Resilience**: Configures `RestartPolicy: { Name: 'unless-stopped' }` so active workspaces survive host machine reboots.

### 3. Container Teardown (`DELETE /containers/:id`)
- Gracefully stops and purges workspace containers, freeing virtual framebuffers and memory.

---

## REST API Endpoints

| Route | Method | Description |
| :--- | :--- | :--- |
| `/health` | `GET` | Host Agent daemon health check |
| `/containers` | `GET` | List all running and stopped ShadowOps workspace containers |
| `/containers` | `POST` | Spawn a new workspace container with specified name and port |
| `/containers/:id` | `GET` | Inspect container status, state, and port mappings |
| `/containers/:id` | `DELETE` | Stop and remove a workspace container |
