# Automation Harness (`run.sh`)

The ShadowOps developer harness provides a unified local execution environment that launches all three platform subsystems concurrently.

## Execution Flow

When invoked (`./run.sh`), the script performs the following background process initializations:
1. **Host Agent**: Bootstraps the Express container daemon (`node index.js`) listening on port `8081`.
2. **Go Backend**: Compiles and launches the core Go API server (`go run .`) listening on port `8080`.
3. **Angular Frontend**: Serves the rich Angular 19 single-page web app (`npm start`) proxying backend connections on port `4200`.

## Process Group Management

The harness aggregates pre-formatted standard output logs (`stdout`) from all three subsystems into a single side-by-side terminal stream. It registers global POSIX signal traps (`trap cleanup SIGINT SIGTERM`) to ensure reliable, zero-orphan process group cleanup upon a single `Ctrl+C`.
