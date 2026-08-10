# Workspace Agent Architecture (`workspace-agent/`)

The Workspace Agent is a Golang service running inside an isolated Docker container (`shadowops-workspace-agent:dev`). It powers real-time browser automation, multi-tab CDP screencasting, ActionSet capture, and dynamic Python Playwright script execution.

---

## Core Component Structure

- **`main.go`**: Service entrypoint with CLI flags (`-port`, `-ws-id`, `-mock`). Initializes HTTP server and engine.
- **`server.go` / `server_test.go`**: HTTP REST API and WebSocket broadcaster:
  - `GET /health` & `GET /status`: Container health checks, viewport bounds, and lifecycle status.
  - `POST /navigate`: Browser navigation to a specified URL.
  - `POST /input/dispatch`: Coordinate mouse clicks, double clicks, and keyboard text typing.
  - `POST /shadow/start` & `POST /shadow/end`: Interactive ActionSet recording and export.
  - `POST /script/run`, `GET /script/status`, `POST /script/cancel`: Dynamic Python Playwright script execution.
  - `GET /events`: WebSocket stream broadcasting live JPEG frames (10+ FPS) and stdout logs.
- **`engine.go`**: Defines the `AgentEngine` interface contract.
- **`real_engine.go`**:
  - Playwright-Go Chromium launcher with virtual framebuffer support (Xvfb).
  - **Dynamic CDP Port**: Allocates `AGENT_PORT + 1000` (e.g. `49152`) per container to prevent port collisions on host network mode.
  - **Isolated User Data Directory**: Uses `/tmp/shadowops-user-data-<workspaceID>` for profile persistence.
  - **Dynamic Multi-Tab Active Streaming**: `getActivePage()` continuously inspects `context.Pages()` and captures frames from the frontmost active tab, seamlessly supporting popups and OAuth redirects.
- **`mock_engine.go` / `mock_engine_test.go`**: Headless mock implementation for unit testing without browser binaries.
- **`runner_template.go` / `runner_template_test.go`**: Embedded Python Playwright CDP harness that executes user scripts against the running browser instance.
- **`state.go` / `state_test.go`**: Thread-safe state manager for workspace metadata and action history.
- **`scripts/`**: Client-side JavaScript hooks injected into pages:
  - `active_element.js`: Element focus tracking.
  - `dom_recorder.js`: DOM event listeners for ActionSet generation.
  - `element_from_point.js`: Interactive coordinate mapping.
  - `stealth_init.js`: Bot detection evasion and navigator property masking.

---

## Dynamic Python Script Execution Model

When executing a Python Playwright script inside the container, the runner harness connects directly via CDP (`http://127.0.0.1:<cdpPort>`) and executes the user's `async def run(...)` function:

```python
import asyncio

async def run(page, context, browser, inputs, helpers):
    """
    Parameters:
      - page: playwright.async_api.Page - The frontmost active browser page
      - context: playwright.async_api.BrowserContext - The persistent browser context
      - browser: playwright.async_api.Browser - The browser connected over CDP
      - inputs: dict - Key-value parameters passed from UI or CSV row
      - helpers: ScriptHelpers - Utility class for logging and delays
    """
    target_url = inputs.get('URL') or inputs.get('url') or 'https://example.com'
    helpers.info(f"Navigating to {target_url}...")

    await page.goto(target_url, wait_until="domcontentloaded")
    await helpers.sleep(1)

    # Perform DOM interactions
    submit_btn = page.locator("button[type='submit']").first
    if await submit_btn.is_visible(timeout=3000):
        await submit_btn.click()
        helpers.info("Clicked submit button.")

    return {
        "status": "succeeded",
        "result": "Task completed successfully!"
    }
```

### ScriptHelpers Utility Methods
- `helpers.info(message)`: Emits an info-level log entry streamed to the frontend.
- `helpers.warn(message)`: Emits a warning-level log entry.
- `helpers.error(message)`: Emits an error-level log entry.
- `await helpers.sleep(seconds)`: Non-blocking asynchronous delay.
