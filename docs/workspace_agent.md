# Workspace Agent Architecture

The Workspace Agent is a Golang service running inside an isolated Dockerized browser container. It provides high-performance real-time browser automation, CDP screencasting, ActionSet capture, and dynamic script execution.

## Core Component Structure

- **`main.go`**: Service entrypoint with configurable CLI flags (`-port`, `-ws-id`, `-mock`).
- **`server.go` / `server_test.go`**: HTTP REST API and WebSocket events broadcaster.
  - `GET /health` & `GET /status`: Container health checks and lifecycle status.
  - `POST /navigate`: Browser navigation to a specified URL.
  - `POST /shadow/start` & `POST /shadow/end`: Interactive ActionSet capture.
  - `POST /script/run`, `GET /script/status`, `POST /script/cancel`: Dynamic Python CDP script execution.
  - `GET /events`: WebSocket stream broadcasting live JPEG frames and log messages.
- **`engine.go`**: Defines the `AgentEngine` interface contract.
- **`real_engine.go`**: Playwright-Go Chromium launcher with virtual framebuffer support, stealth injection, multi-tab lifecycle management, and live CDP frame streaming.
- **`mock_engine.go` / `mock_engine_test.go`**: Headless mock implementation for lightweight testing without browser binaries.
- **`runner_template.go` / `runner_template_test.go`**: Embedded Python Playwright CDP runner harness for dynamic automation scripts.
- **`state.go` / `state_test.go`**: Thread-safe state manager for workspace metadata and action history.
- **`scripts/`**: Client-side JavaScript hooks:
  - `active_element.js`: Element focus tracking.
  - `dom_recorder.js`: DOM event listeners for ActionSet generation.
  - `element_from_point.js`: Interactive coordinate mapping.
  - `stealth_init.js`: Bot detection evasion scripts.

## Dynamic Script Execution Model

When executing a Python Playwright script within the Workspace Agent, the code exposes an `async def run(...)` function:

```python
async def run(page, context, browser, inputs, helpers):
    # page: playwright.async_api.Page - The active browser page instance
    # context: playwright.async_api.BrowserContext - The isolated browser context
    # browser: playwright.async_api.Browser - The browser instance
    # inputs: dict - Dictionary containing row columns or UI inputs (e.g., {'URL': 'https://...'})
    # helpers: ScriptHelpers - A utility class injected for logging and waits

    target_url = inputs.get('URL') or inputs.get('url') or ''
    helpers.info(f"Opening target URL: {target_url}")

    if target_url:
        await page.goto(target_url, wait_until="domcontentloaded")
        await helpers.sleep(0.5)

    return {
        "status": "succeeded",
        "message": "Script executed successfully!"
    }
```

### ScriptHelpers
The `helpers` object provides real-time log streaming:
- `helpers.log(message, level="INFO")`: Emits a timestamped log entry.
- `helpers.info(message)`: Emits an info-level log.
- `helpers.warn(message)`: Emits a warning-level log.
- `helpers.error(message)`: Emits an error-level log.
- `await helpers.sleep(seconds)`: Non-blocking asynchronous delay.
