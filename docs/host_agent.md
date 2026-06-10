# Host Agent Architecture

The Host Agent is a Node.js Express API service running directly on the Compute Engine (GCE) host VM. It acts as the local container orchestration daemon for the ShadowOps platform.

## Responsibilities

1. **Docker Engine Management (`index.js`)**
   - Connects directly to the local host daemon via `/var/run/docker.sock`.
   - Executes non-blocking asynchronous Docker commands (`listContainers`, `getContainer`, `inspect`).

2. **Workspace Container Invocation**
   - Implements `POST /containers` to dynamically spin up isolated Workspace container instances.
   - Binds containers to host networking (`--network host`) and assigns dynamic listener ports (`AGENT_PORT`) for seamless live browser streaming.

3. **Lifecycle & Cleanup**
   - Implements `DELETE /containers/:id` to gracefully stop and purge finished or abandoned workspace browser instances.
