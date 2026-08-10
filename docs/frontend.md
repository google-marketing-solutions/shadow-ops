# Frontend Application Architecture (`frontend/`)

The ShadowOps frontend is a rich Angular 19 single-page web application providing the operator control center for live remote browser automation, batch orchestration, and workflow recording.

---

## Key Interface Components

### 1. Workspaces Dashboard (`dashboard-page.component.ts`)
- **Workspace Cards**: Displays all provisioned workspaces with creation dates, status indicators, and current navigation URLs.
- **Dynamic Renaming**: Inline editable workspace title headers (`PATCH /api/v1/workspaces/:id`).
- **Provisioning Modal**: Instant workspace creation with customizable labels and initial URLs.

### 2. Live Workspace Canvas Viewport (`workspace-page.component.ts`)
- **WebSocket Streaming**: Connects directly to `/api/v1/workspaces/:id/events` to stream 10+ FPS JPEG frames.
- **Coordinate Scaling**: Translates operator viewport mouse clicks, double clicks, scrolls, and drag events to remote Chromium browser dimensions.
- **Interactive Navigation Bar**: Live URL bar with forward/back controls and page reloads.

### 3. Batch Script Runner & Telemetry (`workspace-page.component.ts`)
- **CSV Data Input**: Interactive CSV file upload and table preview with row-level status indicators.
- **API Upload Modal**: Provides the workspace's unique `API Secret Key` and ready-to-use `cURL` commands for programmatic CSV uploading.
- **Execution Telemetry**: Streams structured log entries in real-time alongside visual pass/fail pills and traceback error blocks.
- **Results Export**: Download executed CSV files enriched with returned script values and error messages.

### 4. ActionSet (Shadow) Studio & Gemini AI
- **Action Timeline**: Renders real-time recorded user interactions as labeled step cards.
- **DOM Inspector**: Visualizes element bounding boxes, CSS/XPath selector candidates, and click coordinates.
- **AI Script Synthesis**: Generates full Python Playwright scripts with a single click using Google Gemini.

---

## Technical Stack & Aesthetics

- **Framework**: Angular 19 (Signals, Standalone Components, Zoneless Change Detection).
- **Styling**: Google Material Symbols, CSS Grid / Flexbox, SCSS design tokens with dark/light mode themes.
- **Production Build**: Compiled into `./frontend/dist/frontend/browser` and embedded directly in the Go backend binary.
