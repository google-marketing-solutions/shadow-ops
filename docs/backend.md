# ShadowOps Backend Architecture

The ShadowOps backend is a Go-based API service responsible for workspace orchestration, live stream proxying, action recording, and script execution.

## Core Responsibilities

1. **Workspace Lifecycle (`workspace.go`, `docker.go`, `gce.go`)**
   - Orchestrates remote browser workspaces inside isolated Docker containers.
   - Manages Compute Engine (GCE) VM pools for hybrid cloud execution.
   - **Unique Naming & Renaming**: Enforces mandatory, case-insensitive unique workspace naming on creation (`POST /workspaces`) and in-place editing (`PATCH /workspaces/:id`). Audits existing non-killed sessions and surfaces `"REPETITIVE_WORKSPACE_NAME"` Bad Request errors if duplicate names are chosen.

2. **Live Browser Streaming (`websocket.go`)**
   - Proxies live CDP / WebSocket connections directly to Dockerized Chromium instances.
   - Supports real-time low-latency interaction and visual feedback.

3. **ActionSet / Shadow Capture (`actionset.go`, `handlers_actionset.go`)**
   - Records user actions (clicks, inputs, navigations) as structured JSON steps.
   - Preserves DOM hierarchy, selectors, and user-defined labels for robust code generation.

4. **Playwright Script Execution (`handlers_workspace.go`)**
   - Injects user-authored Python Playwright code into the active workspace container.
   - Streams execution logs, traceback errors, and status back to the frontend.

5. **AI Integration (`gemini.go`)**
   - Connects to Gemini models for automated Playwright code authoring and query generation based on recorded ActionSets.
