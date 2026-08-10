# ShadowOps Backend Architecture (`backend/`)

The ShadowOps backend is a high-performance Go service (Go 1.24, `net/http`) responsible for workspace orchestration, live WebSocket proxying, batch scheduling, and script execution.

---

## Core Responsibilities

### 1. Workspace Lifecycle & Management (`handlers_workspace.go`, `docker.go`)
- **Container Provisioning**: Sends requests to the Host Agent (`POST /containers`) to spin up isolated `shadowops-workspace-agent:dev` Docker containers.
- **Dynamic Port Allocation**: Assigns dedicated agent and CDP debugging ports per container to prevent collisions on host network mode.
- **Health Verification**: Polls workspace health (`GET /health`) with exponential backoff before marking containers active.
- **Workspace Renaming & Metadata**: Supports dynamic in-place workspace renaming (`PATCH /api/v1/workspaces/:id`) and persistent state management.

### 2. Live Browser Streaming Proxy (`websocket.go`)
- **WebSocket Gateway**: Transparently proxies WebSocket connections (`/api/v1/workspaces/:id/events`) between the Angular frontend and the Workspace Agent.
- **Live Canvas Screencasting**: Delivers real-time JPEG frames from Chromium (Playwright-Go) to the browser UI.
- **Input Event Forwarding**: Forwards operator mouse clicks, key presses, scrolling, and coordinate events into the active browser page.

### 3. ActionSet & Shadow Mode Studio (`actionset.go`, `handlers_actionset.go`)
- **Interaction Capture**: Collects user interactions (clicks, keyboard input, navigations) as structured JSON ActionSets.
- **DOM Selector Metadata**: Preserves CSS/XPath selectors, bounding boxes, element tags, and user labels for deterministic replay.

### 4. Dynamic Python Script Runner (`handlers_workspace.go`)
- **Script Dispatch**: Forwards user-authored Python Playwright code (`POST /api/v1/workspaces/:id/script/run`) to the Workspace Agent over CDP.
- **Real-Time Telemetry**: Buffers and streams stdout logs, execution states (`running`, `succeeded`, `failed`), and Python stack traces.
- **Cancellation**: Supports immediate script aborts (`POST /api/v1/workspaces/:id/script/cancel`) via process signal traps.

### 5. Batch CSV Engine & Cron Scheduler (`cron.go`, `handlers_cron.go`)
- **CSV Data Input**: Manages uploaded input datasets (`data/csv/`) and sequential multi-row script execution.
- **Programmatic Hot-Swapping**: Exposes API Key authenticated CSV upload endpoints (`POST /api/v1/workspaces/:id/upload-csv`).
- **Cron Jobs**: Evaluates 5-field standard cron expressions to execute automated workflows on recurring intervals.

### 6. AI Code Generation (`gemini.go`)
- **Gemini Integration**: Calls Google Gemini models (`gemini-2.5-pro`) to synthesize production-ready Python Playwright scripts from recorded ActionSet JSON timelines.

### 7. Pre-Packaged Default Scripts (`default-playwright-scripts/`)
- **Built-In Automation Templates**: Serves baseline automation scripts located in `backend/default-playwright-scripts/` to the UI via `GET /api/v1/default-scripts`.
- **UI Integration**: Adding standard `.py` Playwright automation scripts into the `backend/default-playwright-scripts/` directory automatically makes them visible and selectable in the workspace UI script dropdown without manual file uploading.
- **Docker Packaging**: The Docker build runtime creates `/app/backend/default-playwright-scripts` and optionally copies any local default scripts into the image without requiring scripts to be pre-committed.

---

## REST API Endpoints

| Route | Method | Description |
| :--- | :--- | :--- |
| `/api/v1/health` | `GET` | Service health and feature flags |
| `/api/v1/workspaces` | `GET` | List all workspace records |
| `/api/v1/workspaces` | `POST` | Launch a new workspace container |
| `/api/v1/workspaces/:id` | `GET` | Get workspace details, URL, and API key |
| `/api/v1/workspaces/:id` | `PATCH` | Rename a workspace |
| `/api/v1/workspaces/:id` | `DELETE` | Stop and purge a workspace container |
| `/api/v1/workspaces/:id/events` | `GET` | WebSocket live video & event stream |
| `/api/v1/workspaces/:id/script/run` | `POST` | Launch a script execution |
| `/api/v1/workspaces/:id/script/status` | `GET` | Poll active script execution status & logs |
| `/api/v1/workspaces/:id/script/cancel` | `POST` | Abort a running script |
| `/api/v1/workspaces/:id/upload-csv` | `POST` | Upload CSV dataset via API Key |
| `/api/v1/workspaces/:id/cron` | `GET` / `POST` | Manage automated cron schedules |
| `/api/v1/default-scripts` | `GET` | List pre-packaged default Playwright scripts |
| `/api/v1/actionsets` | `GET` / `POST` | List and save recorded ActionSets |
| `/api/v1/generate-script` | `POST` | Generate Python Playwright script via Gemini AI |
