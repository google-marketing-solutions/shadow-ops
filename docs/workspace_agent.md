# Workspace Agent Architecture

The Workspace Agent is a Python service running inside the isolated Dockerized browser container. It acts as the bridge between the external Go backend and the internal Chromium browser instance.

## Container Files Structure

- **`main.py`** (formerly `real_agent.py`): The core execution server.
  - Launches Chromium via Playwright inside an Xvfb virtual framebuffer.
  - Injects stealth scripts to prevent bot detection tokens (`navigator.webdriver`).
  - Records user interactions via Chrome DevTools Protocol (CDP) for ActionSet capture.
  - Executes dynamic Python Playwright code uploaded by the user, streaming traceback errors and execution stdout back to the backend.

- **`cdp_overlay.py`** (formerly `agent.py`): Standalone CDP frame annotator service.
  - Intercepts raw CDP screencast frames and uses Pillow (PIL) to draw visual status overlays, recording banners, and cursor tracking indicators directly onto the stream.

- **`Dockerfile` & `requirements.txt`**
  - Packages the container on top of the official Microsoft Playwright Ubuntu Noble image.
  - Configures headless/headful execution flags and dynamic listener ports.

## Python Script Structure

When executing a Python Playwright script within the Workspace Agent, the code must expose an `async def run(...)` function with the following signature:

```python
async def run(page, context, browser, target_url, helpers):
    # page: playwright.async_api.Page - The active browser page instance
    # context: playwright.async_api.BrowserContext - The isolated browser context
    # browser: playwright.async_api.Browser - The browser instance
    # target_url: str - The URL to navigate to (if applicable)
    # helpers: ScriptHelpers - A utility class injected for logging and waits

    # 1. Log progress
    helpers.info(f"Opening target URL: {target_url}")
    
    # 2. Interact with the browser
    if target_url:
        await page.goto(target_url, wait_until="domcontentloaded")
        await helpers.sleep(0.5)

    # 3. Return structured results
    return {
        "status": "success",
        "message": "Script executed successfully!"
    }
```

### ScriptHelpers
The `helpers` object provides the following utility methods:
- `helpers.log(message, level="INFO")`: Emits a log entry back to the backend.
- `helpers.info(message)`: Equivalent to `log(message, "INFO")`.
- `helpers.warn(message)`: Equivalent to `log(message, "WARN")`.
- `helpers.error(message)`: Equivalent to `log(message, "ERROR")`.
- `await helpers.sleep(seconds)`: Safe non-blocking wait using `asyncio.sleep`.
