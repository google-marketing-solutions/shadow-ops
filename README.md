# ShadowOps — Agentic Browser Automation & Workspace Platform

[![Go Version](https://img.shields.io/badge/Go-1.24-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Node Version](https://img.shields.io/badge/Node.js-20.x-339933?style=flat&logo=node.js)](https://nodejs.org/)
[![Angular Version](https://img.shields.io/badge/Angular-19.0-DD0031?style=flat&logo=angular)](https://angular.dev/)
[![Playwright](https://img.shields.io/badge/Playwright-v1.51-2EAD33?style=flat&logo=playwright)](https://playwright.dev/)
[![Docker Compose](https://img.shields.io/badge/Docker-Compose_v2-2496ED?style=flat&logo=docker)](https://www.docker.com/)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

**ShadowOps** is an open-source, enterprise-grade agentic browser automation platform. It enables teams to launch isolated, persistent browser workspaces, record live user interactions into deterministic ActionSets, stream low-latency canvas viewports over WebSockets, and execute dynamic Python Playwright scripts at scale via batch CSV processing, cron scheduling, or REST APIs.

---

## 🌟 Key Features

- **🖥️ Live Interactive Viewport**: Stream low-latency browser sessions (10+ FPS JPEG over WebSockets) with full mouse, keyboard, scroll, and resize interaction forwarding.
- **📑 Dynamic Multi-Tab Streaming**: Automatically detects and streams whichever browser tab is active/frontmost, seamlessly supporting popups, OAuth logins, and multi-window workflows.
- **🤖 ActionSet Recording (Shadow Mode)**: Record browser workflows into normalized, structured JSON steps with robust DOM selector scoring, element coordinates, and user-defined labels.
- **⚡ Dynamic Script Runner**: Execute user-authored Python Playwright scripts directly inside isolated Chromium containers over Chrome DevTools Protocol (CDP).
- **📊 Batch CSV Processing**: Execute automated workflows row-by-row against CSV datasets, collecting structured output attributes and tracebacks directly into downloadable result spreadsheets.
- **⏰ Built-In Cron Scheduler**: Schedule automated headless or headed browser runs using standard cron expressions with persistent execution history in SQLite.
- **🔒 Enterprise Security & IAP**: Full compatibility with Google Cloud Identity-Aware Proxy (IAP) TCP Tunneling and HTTPS Load Balancing for Zero Trust access control.
- **🐳 Single-Command Docker Deployment**: Ready-to-use Docker Compose stack with isolated networking, persistent profiles, and automated health checks.

---

## 🏗️ Architecture Overview

ShadowOps is divided into four integrated, decoupled subsystems:

```
                          ┌────────────────────────┐
                          │  Operator Browser (UI) │
                          │  http://localhost:8080 │
                          └───────────┬────────────┘
                                      │
                         HTTP & WS    │ :8080 (REST + Live Canvas)
                                      ▼
                        ┌───────────────────────────┐
                        │   ShadowOps Backend (Go)  │
                        │   • Core REST API Server  │
                        │   • SQLite State & DB     │
                        │   • Cron & Batch Runner   │
                        │   • Angular Web Hosting   │
                        └─────┬───────────────┬─────┘
                              │               │
     HTTP REST (/containers)  │               │ HTTP / WebSocket (:8765+ dynamic)
                              ▼               ▼
         ┌──────────────────────────┐   ┌───────────────────────────┐
         │     Host Agent (Node.js) │   │  Workspace Agent Container│
         │  (Docker Socket Manager) │   │  • Go / Playwright Engine │
         │        Port :8081        │   │  • Python CDP Script Exec │
         └─────────────┬────────────┘   │  • Live Frame Screencaster│
                       │ Launches       │  • Headless/Xvfb Chromium │
                       ▼                └───────────────────────────┘
          [ Docker Engine / Daemon ]
```

### Component Breakdown

1. **`backend/` (Go 1.24)**:
   - Central API server (`net/http`) serving REST endpoints and proxying WebSocket streams.
   - SQLite database (`data/shadowops.db`) for workspace metadata, action sets, and cron jobs.
   - Automatic host discovery (`host.docker.internal`) and workspace health orchestrator.
2. **`host-agent/` (Node.js 20 + Dockerode)**:
   - Local Docker daemon supervisor listening on `:8081`.
   - Spawns and stops workspace containers with `RestartPolicy: 'unless-stopped'` and host networking.
3. **`workspace-agent/` (Go + Playwright)**:
   - Dedicated service running inside each workspace container (`shadowops-workspace-agent:dev`).
   - Launches Chromium (via Xvfb virtual framebuffer) with persistent context and stealth scripts.
   - Streams live JPEG frames, handles coordinate input dispatch, and executes Python Playwright scripts over CDP.
4. **`frontend/` (Angular 19 + TypeScript)**:
   - Reactive dashboard, live session canvas viewport, ActionSet studio, and batch CSV execution manager.

---

## 🚀 Quickstart with Docker Compose

The fastest way to deploy ShadowOps locally or on a cloud VM is using Docker Compose:

### Prerequisites
- [Docker Engine](https://docs.docker.com/engine/install/) (v24.0+)
- [Docker Compose](https://docs.docker.com/compose/install/) (v2.0+)

### 1. Clone the Repository
```bash
git clone https://github.com/your-org/shadow-ops.git
cd shadow-ops
```

### 2. Launch All Services
```bash
docker compose up -d --build
```

### 3. Open the Dashboard
Navigate to **[http://localhost:8080](http://localhost:8080)** in your browser.

- Click **"New Workspace"** to launch an isolated browser session.
- Log in to your target web application (e.g. Google Ads, Salesforce, AWS Console).
- Upload a script or CSV file and start automating!

---

## 🛠️ Local Development Setup

To run ShadowOps directly on your host machine for development:

### Prerequisites
- **Go**: 1.24+ (`go version`)
- **Node.js**: 20.x+ & npm (`node -v`)
- **Python**: 3.10+ with `playwright` (`pip install playwright && playwright install chromium`)

### Unified Startup Harness (`run.sh`)
The repository includes a concurrent multi-process startup script:

```bash
chmod +x run.sh
./run.sh
```

This concurrently boots:
- **Host Agent** on `http://127.0.0.1:8081`
- **Go Backend** on `http://127.0.0.1:8080`
- **Angular Dev Server** on `http://127.0.0.1:4200`

Press `Ctrl+C` to cleanly terminate all processes without leaving orphan containers or background ports open.

---

## 📝 Writing Playwright Automation Scripts

ShadowOps runs user-authored Python Playwright scripts inside the workspace container. Scripts must expose an asynchronous `run(...)` function:

```python
import asyncio

async def run(page, context, browser, inputs, helpers):
    """
    Parameters:
      - page: playwright.async_api.Page (active browser tab)
      - context: playwright.async_api.BrowserContext (persistent browser context)
      - browser: playwright.async_api.Browser (browser instance connected over CDP)
      - inputs: dict (key-value pairs passed from UI or CSV row)
      - helpers: ScriptHelpers (utility class for logging and delays)
    """
    account_id = inputs.get("account_id", "")
    helpers.info(f"Starting automation for account: {account_id}")

    # Navigate or interact with DOM elements
    target_url = "https://ads.google.com/aw/overview"
    await page.goto(target_url, wait_until="domcontentloaded")
    await helpers.sleep(2)

    # Perform actions
    account_row = page.locator(f"tr:has-text('{account_id}')").first
    if await account_row.is_visible(timeout=5000):
        await account_row.click()
        helpers.info("Account row clicked successfully.")

    # Return structured results to populate CSV output
    return {
        "status": "succeeded",
        "processed_account": account_id,
        "verified": True
    }
```

### ScriptHelpers Methods
The injected `helpers` object streams real-time telemetry to the dashboard:
- `helpers.info(msg)`: Emits an `INFO` level log.
- `helpers.warn(msg)`: Emits a `WARN` level log.
- `helpers.error(msg)`: Emits an `ERROR` level log.
- `await helpers.sleep(seconds)`: Non-blocking asynchronous delay.

---

## 📊 Batch CSV Execution

ShadowOps includes a native batch processing engine designed for executing Playwright automations at scale across parameterized datasets:

- **Row-to-Script Parameter Injection**: Each row in an uploaded `.csv` file represents an individual execution unit. The CSV column headers are automatically parsed and passed directly into the Playwright script's `inputs` dictionary argument (`inputs['column_name']`).
- **Sequential Multi-Row Processing**: The platform iterates through each row sequentially within the active workspace session, allowing scripts to perform complex navigations, form submissions, and UI configurations without needing to re-authenticate for every entry.
- **Dynamic Result Enrichment**: Any dictionary values returned by the Playwright `run(...)` function (e.g. status flags, before/after values, generated IDs, verification timestamps) are automatically appended as new columns to that row in the dataset.
- **Real-Time Row Telemetry & Error Handling**: The dashboard renders a live row-by-row execution status table and console stream. If an individual row encounters an exception or timeout, the traceback is captured, recorded to the row's output, and execution gracefully proceeds to the next row.
- **Export Enriched Datasets**: Once batch processing completes, operators can download the finalized CSV containing all original input columns alongside the appended execution results, timestamps, and error summaries.

### 🔑 Programmatic CSV Upload via API Key & cURL

You can hot-swap or upload a workspace's batch CSV dataset programmatically without using the web UI:

1. **Obtain the Workspace API Key**:
   - **From the UI**: Open the workspace in your browser and click the **"API Upload"** button next to the CSV file input to reveal the workspace's secret key and pre-filled command.
   - **From the API**: Call `GET /api/v1/workspaces/{workspace_id}` to retrieve the `"apiKey"` field (`sk_...`).

2. **Upload CSV with cURL**:
   ```bash
   curl -X POST "http://localhost:8080/api/v1/workspaces/<WORKSPACE_ID>/upload-csv" \
     -H "Authorization: Bearer <API_KEY>" \
     -F "file=@/path/to/your_dataset.csv"
   ```

3. **API Response**:
   ```json
   {
     "success": true
   }
   ```

> [!TIP]
> If a Cron automation schedule is configured on the workspace, uploading a new CSV immediately updates the dataset for the next recurring execution.

---

## 🌐 REST API Reference

ShadowOps exposes a full REST API for programmatic orchestration:

| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET` | `/api/v1/health` | Service health status and feature flags |
| `GET` | `/api/v1/workspaces` | List all active and stopped workspaces |
| `POST` | `/api/v1/workspaces` | Create and launch a new workspace container |
| `GET` | `/api/v1/workspaces/{id}` | Get workspace status, current URL, and API key |
| `PATCH` | `/api/v1/workspaces/{id}` | Rename a workspace |
| `DELETE` | `/api/v1/workspaces/{id}` | Stop and remove a workspace container |
| `POST` | `/api/v1/workspaces/{id}/script/run` | Execute a Python script with JSON inputs |
| `GET` | `/api/v1/workspaces/{id}/script/status` | Get active script run status and logs |
| `POST` | `/api/v1/workspaces/{id}/script/cancel` | Terminate active script execution |
| `POST` | `/api/v1/workspaces/{id}/upload-csv` | Programmatically upload/replace workspace CSV |
| `GET` | `/api/v1/workspaces/{id}/cron` | Get workspace cron schedule and execution history |
| `POST` | `/api/v1/workspaces/{id}/cron` | Set or update workspace cron automation schedule |
| `GET` | `/api/v1/default-scripts` | List built-in default Playwright scripts in `backend/default-playwright-scripts/` |

---

## ⚙️ Environment Variables

Configure ShadowOps using environment variables or a `.env` file:

| Variable | Default | Description |
| :--- | :--- | :--- |
| `PORT` | `8080` | HTTP port for the Backend server |
| `BIND_ADDR` | `:8080` | Network address for backend binding |
| `HOST_AGENT_URL` | `http://127.0.0.1:8081` | Base URL to reach the Host Agent daemon |
| `DATA_DIR` | `./data` | Local directory for SQLite DB and CSV artifacts |
| `STATIC_DIR` | `./frontend/dist/frontend/browser` | Path to compiled Angular static UI files |
| `WORKSPACE_IMAGE_NAME` | `shadowops-workspace-agent:dev` | Docker image tag for workspace containers |
| `SHADOWOPS_ENABLED` | `true` | Enables ActionSet recording & custom script authoring |
| `GEMINI_API_KEY` | *(optional)* | Google Gemini API key for AI script generation |
| `GEMINI_MODEL` | `gemini-2.5-pro` | Gemini model name for code authoring |

---

## ☁️ Cloud Deployment & Security

For production deployments on **Google Cloud Platform (GCP)**:
- Deploy the Docker Compose stack onto a **Google Compute Engine (GCE)** Ubuntu VM.
- Secure access using **Identity-Aware Proxy (IAP)** to ensure Zero Trust Google SSO authentication without exposing public ports.

Detailed instructions are available in [**`deployment.md`**](./deployment.md).

---

## 🤝 Contributing

Contributions are welcome! Please follow these steps:
1. Fork the repository.
2. Create a feature branch: `git checkout -b feature/my-enhancement`.
3. Ensure all tests pass: `go test ./...` and `ng test`.
4. Submit a pull request.

---

## 📄 License

ShadowOps is licensed under the [Apache License 2.0](https://www.apache.org/licenses/LICENSE-2.0).
