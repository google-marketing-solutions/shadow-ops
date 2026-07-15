# ShadowOps UI Features

The ShadowOps frontend (built in Angular 17) provides a unified control center for managing browser automation workspaces.

## 1. Dashboard
- **Workspace Listing**: Displays all active and stopped workspaces along with their creation dates and status.
- **Workspace Creation**: Easily provision a new isolated browser automation workspace by providing a custom name.

## 2. Workspace Control Center
The workspace page is the heart of ShadowOps, providing live interaction and script orchestration for a specific browser session.

### Live Session Viewport
- Real-time video stream of the Playwright browser.
- Interactive URL navigation bar to direct the browser session securely.
- Click and scroll event forwarding to interact directly with the web page from the UI.

### Session Controls
- **Recording Modes**: Toggle between 'Manual Mode' (user-controlled) and 'Recording Mode' (interactions are normalized into shadow steps).
- **Emergency Kill**: Forcefully terminates the Docker container running the workspace.
- **Network Engine Indicator**: Shows WebSocket health and reconnection status.

### Batch Script Runner
- **CSV Data Input**: Upload a CSV file directly through the UI to run automation tasks iteratively across the rows.
- **API CSV Upload**: Provides an "API Upload" button that reveals a sleek modal containing the workspace's unique `API Secret Key` and a copy-to-clipboard ready `cURL` command for programmatic CSV hot-swapping.
- **Python Script Loading**: If `SHADOWOPS_ENABLED=true`, users can upload their own custom Playwright Python scripts. Otherwise, they can select from a curated dropdown of default scripts.
- **Run/Cancel Controls**: Manually execute or abort a running batch job.
- **Results Download**: Download the executed CSV which contains the appended output results from the script executions.

### Script Logs & Telemetry
- **Live Logging Panel**: Streams structured log entries directly from the Python script running inside the isolated Docker container.
- **Tracebacks**: Automatically formats and displays Python tracebacks if the script crashes or fails.
- **Execution Status Pill**: Visual indicator tracking whether the script is Idle, Running, Failed, or Succeeded.
