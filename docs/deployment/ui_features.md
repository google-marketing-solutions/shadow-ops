# ShadowOps UI Features

The ShadowOps frontend (built in Angular 19) provides a unified control center for managing browser automation workspaces.

---

## 1. Dashboard
- **Workspace Listing**: Displays all active and stopped workspaces along with creation dates, status indicators, and live URL previews.
- **Workspace Creation**: Easily provision a new isolated browser automation workspace with customizable labels and initial navigation URLs.
- **Inline Renaming**: Rename any workspace in place directly from the dashboard card header.

---

## 2. Workspace Control Center
The workspace page is the heart of ShadowOps, providing live interaction and script orchestration for a specific browser session.

### Live Session Viewport
- **Real-Time Video Stream**: Low-latency JPEG canvas stream (10+ FPS) from the Playwright browser.
- **Dynamic Multi-Tab Streaming**: Automatically switches the canvas view to follow the frontmost active tab when popups or OAuth windows open.
- **Interactive Navigation Bar**: Direct the browser session securely with address bar inputs, reload, and back/forward controls.
- **Interaction Forwarding**: Native click, double-click, scroll, and text typing event dispatching.

### Session Controls
- **Recording Modes**: Toggle between 'Manual Mode' (user-controlled) and 'Recording Mode' (interactions are normalized into shadow steps).
- **Emergency Kill**: Gracefully terminates the Docker container running the workspace with an animated confirmation modal loader before safely returning to the dashboard.
- **Network Engine Indicator**: Visual pill showing WebSocket connection health and reconnection state.

### Automation Orchestration & `+ Create Run` Modal
- **Multi-Step Wizard**: Clicking `+ Create Run` opens a modal allowing operators to select scripts (upload `.py` or load from library) and optional CSV datasets.
- **Immediate vs Scheduled Execution**: Choose between immediate run execution (`▶ Run Now`) or configuring automated cron recurrence (`⏰ Set Schedule`).
- **Cron Details & Dynamic Countdown**: Real-time ticker counting down until the next scheduled batch execution (`⏳ X days, Y hours, Z mins left`).
- **Active Run Cancellation**: `■ Kill Active Run` stop button appears only during active execution, triggering an animated loader and status confirmation upon stopping.
- **API CSV Upload**: Provides an "API Upload" modal containing the workspace's unique `API Secret Key` and copy-to-clipboard ready `cURL` commands for programmatic CSV hot-swapping.
- **Results Download**: Download the executed CSV enriched with returned output attributes and error tracebacks.

### Script Logs & Telemetry
- **Live Logging Panel**: Streams structured log entries directly from the Python script running inside the isolated Docker container.
- **Log Tools**: One-click `📋 Copy Logs` to clipboard and `⛶ Enlarge View` modal for full-screen inspection of high-throughput logs.
- **Execution History & Tracebacks**: Runs table displaying previous batch runs and interactive traceback accordions for failed jobs.
