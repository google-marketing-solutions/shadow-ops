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
- **Emergency Kill**: Gracefully terminates the Docker container running the workspace.
- **Network Engine Indicator**: Visual pill showing WebSocket connection health and reconnection state.

### Batch Script Runner
- **CSV Data Input**: Upload a CSV file directly through the UI to run automation tasks sequentially across rows.
- **API CSV Upload**: Provides an "API Upload" modal containing the workspace's unique `API Secret Key` and copy-to-clipboard ready `cURL` commands for programmatic CSV hot-swapping.
- **Python Script Loading**: If `SHADOWOPS_ENABLED=true`, users can upload custom Playwright Python scripts. Otherwise, they can select from a curated dropdown of default scripts.
- **Run/Cancel Controls**: Manually execute or abort a running batch job with instant feedback.
- **Results Download**: Download the executed CSV enriched with returned output attributes and error tracebacks.

### Script Logs & Telemetry
- **Live Logging Panel**: Streams structured log entries directly from the Python script running inside the isolated Docker container.
- **Tracebacks**: Formats and displays Python tracebacks if the script crashes or encounters an exception.
- **Execution Status Pill**: Real-time visual indicator tracking whether the script is Idle, Running, Failed, or Succeeded.
