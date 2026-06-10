# Frontend Application Architecture

The ShadowOps frontend is a rich Angular 19 single-page web application providing the interactive control center for live remote browser automation.

## Key Interface Components

1. **Live Workspace Stream Panel**
   - Connects over WebSocket directly to the remote browser container over dynamic proxy ports.
   - Renders low-latency screencast video frames and captures direct user interaction (clicking, typing, scrolling).

2. **ActionSet (Shadow) Studio**
   - Displays real-time recorded workflow steps as they occur in the active browser session.
   - Allows users to organize, label, and manage recorded actions with rich DOM selector metadata.

3. **Playwright IDE & Execution Panel**
   - Provides an in-browser Python authoring interface for Playwright automation scripts.
   - Renders live execution stdout streaming and interactive traceback debugging blocks side-by-side with the live browser view.

## Aesthetics & Styling

- Built using Google Material styling and SCSS design tokens with adaptable light/dark mode color schemes.
- Utilizes clean typography and subtle micro-animations for interactive UI state transitions.
