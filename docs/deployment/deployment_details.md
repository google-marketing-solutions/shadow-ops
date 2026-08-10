# ShadowOps Deployment Details

This document outlines the architectural components, configuration flags, and data persistence mechanisms for deploying the ShadowOps platform.

---

## Architecture
ShadowOps is built on an integrated four-part architecture:
1. **Frontend (Angular 19)**: The control center UI hosted on port `4200` (dev) or embedded in the backend binary on `8080` (prod).
2. **Backend (Go 1.24)**: The primary REST API, WebSocket proxy, and Cron Scheduler hosted on port `8080`. Connects to a local SQLite database for state management.
3. **Host Agent (Node.js 20)**: A lightweight Docker orchestration agent hosted on port `8081`. It spawns, manages, and cleans up isolated Docker containers for browser sessions.
4. **Workspace Agent (Go + Playwright)**: An isolated container service (`shadowops-workspace-agent:dev`) running Playwright and Chromium on dynamic ports.

---

## Prerequisites
To deploy or run ShadowOps, the host machine requires:
* **Go** (v1.24+ for the backend API)
* **Node.js & npm** (v20+ for the host agent and Angular frontend)
* **Docker & Docker Compose** (v24.0+ / v2.0+ to spawn ephemeral browser workspaces)

---

## Environment Variables
The platform reads the following environment variables upon startup:

| Variable | Description | Default |
| :--- | :--- | :--- |
| `SHADOWOPS_ENABLED` | Set to `true` to enable custom Python script uploading in the UI. If `false`, users select from predefined default scripts. | `true` |
| `PORT` | The port the Go backend API runs on. | `8080` |
| `HOST_AGENT_URL` | The full URL to reach the Host Agent daemon. | `http://127.0.0.1:8081` |
| `DATA_DIR` | Directory for SQLite DB (`shadowops.db`) and CSV run artifacts. | `./data` |
| `WORKSPACE_IMAGE_NAME` | Docker image tag for workspace containers. | `shadowops-workspace-agent:dev` |

---

## Data Persistence & Resilience
ShadowOps is designed to survive crashes, restarts, and host machine reboots gracefully:

- **Database (SQLite)**: Core configuration (Workspace names, API Keys, Cron Jobs) are stored locally in `data/shadowops.db`. Schema updates are executed dynamically on boot using `ALTER TABLE` migrations.
- **Docker Auto-Restart**: The Node.js Host Agent orchestrates workspace containers with the `--restart unless-stopped` flag. If the host machine reboots, Docker automatically revives active workspaces.
- **Persistent Browser Contexts**: Workspace containers maintain persistent user profiles in `/tmp/shadowops-user-data-<wsID>`. This ensures user sessions, cookies, and logins remain active across restarts.
- **Backend Recovery Check**: Upon startup, the Go backend queries the Host Agent for existing containers and reattaches them, syncing state and preventing orphan containers.

---

## Local Startup Harness
For local execution, the unified `run.sh` script automates dependency checks and concurrent execution:
```bash
./run.sh
```
The script traps `SIGINT`/`SIGTERM` to gracefully teardown child processes upon exit.
