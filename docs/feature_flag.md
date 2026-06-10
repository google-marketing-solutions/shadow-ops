# ShadowOps Feature Flag (`SHADOWOPS_ENABLED`)

This document details the full-stack implementation of the runtime environment feature flag used to toggle advanced ShadowOps platform capabilities.

## 1. Environment Variable Configuration
The platform reads the feature flag from the operating system environment variable:
```bash
export SHADOWOPS_ENABLED="true"
```
By default, if this variable is omitted or set to any value other than `"true"` (case-insensitive), the feature flag defaults to **OFF** (`false`).

## 2. Go API Backend Integration (`server.go`)
- **Initialization**: When `NewServer()` boots, it evaluates `os.Getenv("SHADOWOPS_ENABLED")` and caches the boolean state in `s.shadowopsEnabled`.
- **Exposure**: The public health check endpoint (`GET /api/v1/health`) returns `"shadowopsEnabled": boolean`, allowing UI clients to sync state instantly without requiring authenticated configuration handshakes.

## 3. Angular 19 Interface Toggling & Script Permissions
The Angular frontend polls `/api/v1/health` on boot in `app.component.ts` and stores the state in `SessionStateService.shadowopsEnabled` Signal.

- **When ON (`true`)**: Unlocks custom Python Playwright script authoring/uploading inside the IDE, real-time ActionSet capture studios, interactive DOM selector timelines, and manual recording controls.
- **When OFF (`false`)**: Streamlines the Workspace control panel to render **Batch Execution (CSV) and Cron Schedule alone**. Custom script authoring is disabled; operators select verified automations exclusively from the `"Select Default Playwright Script"` dropdown populated via `GET /api/v1/default-scripts` (`backend/default-playwright-scripts/`).
- **Operational Safeguards**: Requires explicit `confirm()` popups before activating new automated Cron schedules or cancelling active background tasks, surfacing immediate alert acknowledgements upon successful termination.
