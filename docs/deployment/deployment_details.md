# ShadowOps Deployment Details

This document outlines the architectural components, configuration flags, and data persistence mechanisms for deploying the ShadowOps platform.

## Architecture
ShadowOps is built on a unified trifold architecture:
1. **Frontend (Angular 17)**: The control center UI hosted on port `4200`.
2. **Backend (Go 1.22+)**: The primary REST API and Cron Scheduler hosted on port `8080`. Connects to a local SQLite database for state management.
3. **Host Agent (Node.js)**: A lightweight Docker orchestration agent hosted on port `8081`. It spawns, manages, and cleans up isolated Docker containers (workspaces) for executing the actual Python Playwright scripts.

## Prerequisites
To deploy or run ShadowOps, the host machine requires:
* **Go** (for the backend API)
* **Node.js & npm** (for the host agent and Angular frontend)
* **Docker** (to spawn ephemeral browser workspaces)

## Environment Variables
The platform reads the following environment variables upon startup:

| Variable | Description | Default |
| -------- | ----------- | ------- |
| `SHADOWOPS_ENABLED` | Set to `true` to enable custom Python script uploading in the UI. If `false`, users can only select from predefined default scripts. | `false` |
| `PORT` | The port the Go backend API runs on. | `8080` |
| `HOST_AGENT_PORT` | The port the Node.js Host Agent runs on. | `8081` |

## Data Persistence & Resilience
ShadowOps is designed to survive crashes, restarts, and host machine reboots gracefully.

- **Database (SQLite)**: Core configuration (Workspace names, API Keys, Cron Jobs) are stored locally. Schema updates (like adding the `api_key` column) are executed dynamically on boot using `ALTER TABLE` migrations.
- **Docker Auto-Restart**: The Node.js Host Agent orchestrates workspace containers with the `--restart unless-stopped` flag. If the host machine reboots, Docker automatically revives the active workspaces.
- **Persistent Browser Contexts**: Workspace containers launch Playwright using `launch_persistent_context` mapped to internal Docker volumes. This ensures user sessions, cookies, and Google Logins remain active across container restarts.
- **Backend Recovery Check**: Upon startup, the Go backend queries the Host Agent for existing containers and reattaches them, syncing state and preventing orphan containers.

## Local Startup Harness
For local execution, the unified `run.sh` script automates dependency installation and concurrent execution of all three components.
```bash
./run.sh
```
The script traps `SIGINT`/`SIGTERM` to gracefully teardown the sub-processes upon exit.
