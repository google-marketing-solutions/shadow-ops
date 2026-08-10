# ShadowOps Platform Deployment & Configuration Guide (`deployment.md`)

This guide provides end-to-end instructions for deploying, configuring, and running **ShadowOps Phase 1** across **Standalone Servers** (Linux VMs, DigitalOcean Droplets, Bare Metal, Local PCs) and **Google Cloud Platform** (GKE, Cloud Run, Compute Engine).

---

## 1. System Architecture & Port Topology

ShadowOps consists of 4 integrated services:

```
                          ┌────────────────────────┐
                          │   Browser (Client)     │
                          └───────────┬────────────┘
                                      │
                         HTTP & WS    │ :8080 (or :4200 dev)
                                      ▼
                        ┌───────────────────────────┐
                        │   ShadowOps Backend (Go)  │
                        │   + Integrated Web UI     │
                        └─────┬───────────────┬─────┘
                              │               │
     HTTP REST (/containers)  │               │ HTTP / WebSocket (:8765 or dynamic)
                              ▼               ▼
         ┌──────────────────────────┐   ┌───────────────────────────┐
         │     Host Agent (Node.js) │   │  Workspace Agent Container│
         │  (Docker Socket Manager) │   │ (Python/Go + Playwright)  │
         │        Port :8081        │   │       Port :8765+         │
         └─────────────┬────────────┘   └───────────────────────────┘
                       │ Launches
                       ▼
          [ Docker Engine / Daemon ]
```

### Detailed Protocol & URL Resolution Diagrams

#### 1. Standalone / VM / Docker Compose Deployment (HTTP or HTTPS)

```
                       ┌────────────────────────┐
                       │   Client Browser (UI)  │
                       └───────────┬────────────┘
                                   │
                    HTTP / HTTPS   │  :8080 (prod) / :4200 (dev)
                    WS / WSS       │  WebSocket auto-negotiates wss:// if on https://
                                   ▼
                     ┌───────────────────────────┐
                     │   ShadowOps Backend (Go)  │
                     │   + Integrated Web UI     │
                     └─────┬───────────────┬─────┘
                           │               │
  HOST_AGENT_URL           │               │ Auto-managed Container Port
  HTTP / HTTPS REST        │               │ HTTP / WebSocket (:8765 -> localhost)
  (e.g. http://127.0.0.1:8081)             │ (e.g. http://127.0.0.1:49152)
  or https://host.domain:8443)             │
                           ▼               ▼
      ┌──────────────────────────┐   ┌───────────────────────────┐
      │     Host Agent (Node.js) │   │  Workspace Agent Container│
      │  (Docker Socket Manager) │   │ (Python/Go + Playwright)  │
      │   HTTP / HTTPS :8081     │   │   HTTP / WS Port :8765    │
      └─────────────┬────────────┘   └───────────────────────────┘
                    │ Launches via Unix Socket
                    ▼ (/var/run/docker.sock)
       [ Docker Engine / Daemon ]
```

#### 2. Google Cloud / GKE Enterprise Deployment (HTTPS & Cloud Security)

```
                       ┌────────────────────────┐
                       │   Client Browser (UI)  │
                       └───────────┬────────────┘
                                   │
                      HTTPS / WSS  │  :443 (Managed TLS Certificate)
                                   ▼
                     ┌───────────────────────────┐
                     │  Cloud Run / Ingress      │
                     │  (ShadowOps Backend + UI) │
                     └─────┬───────────────┬─────┘
                           │               │
        GKE_ENDPOINT       │               │ Direct Pod / ClusterIP
        HTTPS (Port :443)  │               │ HTTP / WS (Port :8765)
        mTLS + OAuth Token │               │ (Private VPC / Service Mesh)
                           ▼               ▼
      ┌──────────────────────────┐   ┌───────────────────────────┐
      │   GKE Control Plane API  │   │  Workspace Agent Pods     │
      │  (Replaces Host Agent)   │   │  (Playwright + Chrome)    │
      └─────────────┬────────────┘   │   Namespace: K8S_NAMESPACE│
                    │ Provisions     └───────────────────────────┘
                    ▼
       [ GKE Worker Nodes Pool ]
```

### HTTP vs. HTTPS Protocol Resolution Rules

| Communication Channel | Supported Protocols | Configuration Variable | Resolution Behavior |
| :--- | :--- | :--- | :--- |
| **Browser $\rightarrow$ Backend** | `http://` or `https://`<br>`ws://` or `wss://` | `BIND_ADDR` / `PORT` | Auto-detected by browser. If accessed via `https://`, Angular client automatically opens WebSocket over secure `wss://`. |
| **Backend $\rightarrow$ Host Agent** | `http://` or `https://` | `HOST_AGENT_URL`<br>`HOST_AGENT_ADDR`<br>`HOST_AGENT_IP` | • `HOST_AGENT_URL` preserves full scheme (e.g. `https://host-agent.internal:8443`).<br>• `HOST_AGENT_ADDR` without scheme defaults to `http://<addr>`.<br>• Go HTTP client verifies TLS certificates with system CAs. |
| **Backend $\rightarrow$ Workspace Agent** | `http://` and `ws://` | *(Automatic)* | Automatically routed to the local allocated container port (`http://127.0.0.1:<port>`). No manual configuration required. |
| **Backend $\rightarrow$ GKE Cluster** | `https://` (TLS) | `GKE_ENDPOINT`<br>`GKE_CA_CERT` | Communicates over HTTPS to GKE master using base64 CA certificate and GCP metadata OAuth bearer tokens. |
| **Backend $\rightarrow$ Gemini AI** | `https://` (TLS) | `GEMINI_API_ENDPOINT` | Encrypted HTTPS REST calls to Google AI (`generativelanguage.googleapis.com`) with API key or ADC OAuth tokens. |

| Component | Standalone / VM Mode | GKE / Cloud Mode |
| :--- | :--- | :--- |
| **Backend** | Go binary / Docker container (:8080) | Google Cloud Run or GKE Deployment (:8080) |
| **Frontend UI** | Served directly by Backend or ng serve (:4200) | Integrated into Backend container bundle |
| **Host Agent** | Node.js process (:8081) managing Docker socket | **Replaced by Kubernetes API** (not deployed) |
| **Workspace Agent** | Local Docker containers managed by Host Agent | Ephemeral Kubernetes Pods managed by GKE |
| **Orchestrator** | Node.js Dockerode ([host-agent/](file:///usr/local/google/home/shrxx/Documents/shadowops-rewrite/shadow-ops/host-agent)) | GKE Control Plane via [backend/gke.go](file:///usr/local/google/home/shrxx/Documents/shadowops-rewrite/shadow-ops/backend/gke.go) |

---

## 2. Environment Variables Reference

All services support environment variable configuration. Copy [`.env.example`](file:///usr/local/google/home/shrxx/Documents/shadowops-rewrite/shadow-ops/.env.example) to `.env` to customize settings.

### Backend Configuration (`backend/`)

| Variable | Default Value | Description |
| :--- | :--- | :--- |
| `PORT` / `BACKEND_PORT` | `8080` | HTTP port the backend server listens on. |
| `BIND_ADDR` | `:8080` | Network interface address to bind (e.g. `:8080` or `0.0.0.0:8080`). |
| `DATA_DIR` | `./data` | Directory for SQLite DB (`shadowops.db`) and CSV run artifacts. |
| `STATIC_DIR` | `./frontend/dist/frontend/browser` | Path to built Angular UI bundle for integrated web hosting. |
| `HOST_AGENT_URL` | `http://127.0.0.1:8081` | Full HTTP/HTTPS base URL of Host Agent (e.g. `https://host-agent.example.com:8443`). |
| `HOST_AGENT_ADDR` / `HOST_AGENT_IP` | `127.0.0.1:8081` | Host Agent address; supports both `host:port` (prepends `http://`) and explicit `https://` URLs. |
| `WORKSPACE_IMAGE_NAME` | `shadowops-workspace-agent:dev` | Docker image tag launched for local workspace sessions. |
| `DEFAULT_TARGET_URL` | `https://accounts.google.com/...` | Default browser URL when opening new workspace sessions. |
| `SHADOWOPS_ENABLED` | `true` | Enables platform action-recording and batch scheduling features. |

### Gemini AI Configuration

| Variable | Default Value | Description |
| :--- | :--- | :--- |
| `GEMINI_MODEL` | `gemini-2.5-pro` | Gemini model name for Playwright script generation. |
| `GEMINI_API_KEY` | *(empty)* | Optional Gemini API key (defaults to Google Cloud ADC if empty). |
| `GEMINI_API_ENDPOINT` | *(derived from model)* | Custom Gemini API endpoint override. |

### Google Cloud & Kubernetes Configuration (`GKE`)

| Variable | Default Value | Description |
| :--- | :--- | :--- |
| `K8S_NAMESPACE` / `GKE_NAMESPACE` | `default` | Kubernetes namespace where workspace pods are scheduled. |
| `WORKSPACE_POD_IMAGE` | `shadowops-workspace-agent:latest` | Container image in Artifact Registry / GCR for workspace pods. |
| `WORKSPACE_AGENT_PORT` | `8765` | Workspace agent port inside Kubernetes pods. |
| `POD_CPU_REQUEST` | `200m` | Kubernetes CPU request for workspace agent pods. |
| `POD_MEMORY_REQUEST` | `512Mi` | Kubernetes Memory request for workspace agent pods. |
| `POD_CPU_LIMIT` | *(none)* | Optional Kubernetes CPU limit. |
| `POD_MEMORY_LIMIT` | *(none)* | Optional Kubernetes Memory limit. |
| `POD_POOL_LIMIT` | `10` | Maximum number of concurrent workspace pods allowed in pool. |
| `GKE_ENDPOINT` | *(none)* | GKE control plane API endpoint (when backend runs in Cloud Run). |
| `GKE_CA_CERT` | *(none)* | Base64-encoded GKE cluster Certificate Authority certificate. |

### Host Agent Configuration (`host-agent/`)

| Variable | Default Value | Description |
| :--- | :--- | :--- |
| `PORT` | `8081` | HTTP port for Host Agent API. |

> [!TIP]
> **Workspace Agent is Zero-Configuration**: The Workspace Agent container ([workspace-agent/Dockerfile](file:///usr/local/google/home/shrxx/Documents/shadowops-rewrite/shadow-ops/workspace-agent/Dockerfile)) is completely self-contained with bundled Chrome, Playwright, Xvfb, and its internal server. It requires **no manual environment variables or custom configuration**; the platform backend and orchestrator manage all workspace sessions automatically.

---

## 3. Standalone Server Deployment (VM, Droplet, Bare Metal, PC)

### Option A: One-Command Docker Compose (Recommended)

Docker Compose starts the Backend (with integrated Frontend UI), the Host Agent (with Docker daemon socket mounted), and pre-builds the Workspace Agent image.

#### Prerequisites
- Docker Engine 24+ and Docker Compose v2+ installed.

#### Step-by-Step
1. **Clone the repository**:
   ```bash
   git clone <repository_url> shadow-ops
   cd shadow-ops
   ```

2. **Configure Environment** (optional):
   ```bash
   cp .env.example .env
   # Edit .env to set your GEMINI_API_KEY if needed
   ```

3. **Launch the platform**:
   ```bash
   docker compose up -d --build
   ```

4. **Verify running containers**:
   ```bash
   docker compose ps
   ```

5. **Access the Dashboard**:
   Open your browser at `http://<SERVER_IP>:8080`.

---

### Option B: Unified Production Docker Container

You can build and run a standalone all-in-one container that hosts both the Go backend API and compiled Angular frontend.

1. **Build the container image**:
   ```bash
   docker build -t shadowops-server:latest -f Dockerfile .
   ```

2. **Run Host Agent on host or in Docker**:
   ```bash
   cd host-agent && npm install && PORT=8081 node index.js &
   ```

3. **Run Backend Container**:
   ```bash
   docker run -d \
     --name shadowops-backend \
     --restart unless-stopped \
     -p 8080:8080 \
     -v $(pwd)/data:/app/data \
     -e HOST_AGENT_URL="http://host.docker.internal:8081" \
     -e WORKSPACE_AGENT_HOST="host.docker.internal" \
     --add-host=host.docker.internal:host-gateway \
     shadowops-server:latest
   ```

---

### Option C: Native Host Process Deployment (Systemd / Script)

To run directly on the host machine (e.g. Ubuntu 22.04 / 24.04 VM):

1. **Install Dependencies**:
   ```bash
   # Go 1.24+
   sudo snap install go --classic

   # Node.js 20+
   curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash -
   sudo apt-get install -y nodejs

   # Docker
   sudo apt-get install -y docker.io
   sudo usermod -aG docker $USER
   ```

2. **Build Workspace Agent Image**:
   ```bash
   cd workspace-agent
   docker build -t shadowops-workspace-agent:dev .
   cd ..
   ```

3. **Build Frontend**:
   ```bash
   cd frontend
   npm install
   npm run build
   cd ..
   ```

4. **Run All Services via `run.sh`**:
   ```bash
   ./run.sh
   ```

---

## 4. Google Cloud Platform (GCP) Deployment

### Option A: GKE (Google Kubernetes Engine) + Cloud Run

In this enterprise topology, the **Backend + Web UI** runs on Google Cloud Run, and dynamic browser automation agents (**Workspace Agents**) run as isolated pods in a GKE cluster.

> [!NOTE]
> In GKE deployment mode, the **Host Agent** is **not deployed**. The Go backend talks directly to the GKE Control Plane via [backend/gke.go](file:///usr/local/google/home/shrxx/Documents/shadowops-rewrite/shadow-ops/backend/gke.go) to provision and manage Workspace Agent pods.

#### 1. Setup GCP Project & Artifact Registry
```bash
export PROJECT_ID="my-gcp-project"
export REGION="us-central1"
export CLUSTER_NAME="shadowops-cluster"

gcloud config set project $PROJECT_ID

# Enable required APIs
gcloud services enable \
  container.googleapis.com \
  run.googleapis.com \
  artifactregistry.googleapis.com

# Create Artifact Registry repository
gcloud artifacts repositories create shadowops-repo \
  --repository-format=docker \
  --location=$REGION \
  --description="ShadowOps Container Repository"
```

#### 2. Build and Push Workspace Agent Image
```bash
export WORKSPACE_IMAGE="$REGION-docker.pkg.dev/$PROJECT_ID/shadowops-repo/workspace-agent:latest"

docker build -t $WORKSPACE_IMAGE ./workspace-agent
gcloud auth configure-docker $REGION-docker.pkg.dev
docker push $WORKSPACE_IMAGE
```

#### 3. Create GKE Cluster
```bash
gcloud container clusters create-auto $CLUSTER_NAME \
  --region=$REGION \
  --enable-master-authorized-networks=false
```

#### 4. Configure Workload Identity & Cloud Run Service Account
```bash
# Create Service Account for Backend
gcloud iam service-accounts create shadowops-backend-sa \
  --display-name="ShadowOps Backend Service Account"

# Grant Kubernetes Developer role on GKE
gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member="serviceAccount:shadowops-backend-sa@$PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/container.developer"

# Grant Gemini / Vertex AI user role
gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member="serviceAccount:shadowops-backend-sa@$PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/aiplatform.user"
```

#### 5. Deploy Backend to Cloud Run
```bash
# Get GKE cluster endpoint and CA Cert
export GKE_ENDPOINT="https://$(gcloud container clusters describe $CLUSTER_NAME --region=$REGION --format='value(endpoint)')"
export GKE_CA_CERT="$(gcloud container clusters describe $CLUSTER_NAME --region=$REGION --format='value(masterAuth.clusterCaCertificate)')"

# Build and push Backend Image
export BACKEND_IMAGE="$REGION-docker.pkg.dev/$PROJECT_ID/shadowops-repo/backend:latest"
docker build -t $BACKEND_IMAGE -f Dockerfile .
docker push $BACKEND_IMAGE

# Deploy Cloud Run Service
gcloud run deploy shadowops-backend \
  --image=$BACKEND_IMAGE \
  --region=$REGION \
  --service-account="shadowops-backend-sa@$PROJECT_ID.iam.gserviceaccount.com" \
  --set-env-vars="GKE_ENDPOINT=$GKE_ENDPOINT,GKE_CA_CERT=$GKE_CA_CERT,WORKSPACE_POD_IMAGE=$WORKSPACE_IMAGE,K8S_NAMESPACE=default,SHADOWOPS_ENABLED=true" \
  --allow-unauthenticated \
  --port=8080 \
  --cpu=2 \
  --memory=2Gi
```

---

### Option B: Google Compute Engine (GCE) VM

Deploy the complete Docker Compose stack onto a single Compute Engine VM with attached persistent SSD.

```bash
# Create GCE instance with Docker pre-installed
gcloud compute instances create shadowops-vm \
  --zone="us-central1-a" \
  --machine-type="e2-standard-4" \
  --image-family="ubuntu-2404-lts-amd64" \
  --image-project="ubuntu-os-cloud" \
  --boot-disk-size="50GB" \
  --tags="http-server,https-server" \
  --scopes="cloud-platform"

# Allow firewall traffic on port 8080
gcloud compute firewall-rules create allow-shadowops-8080 \
  --allow=tcp:8080 \
  --target-tags="http-server"
```

SSH into the instance and run:
```bash
git clone <repository_url> shadow-ops
cd shadow-ops
docker compose up -d --build
```

---

## 5. Verification & Health Checks

Once deployed, verify that all services are operational:

1. **Backend Health Check**:
   ```bash
   curl -s http://localhost:8080/api/v1/health | jq
   ```
   **Expected Response**:
   ```json
   {
     "ok": true,
     "service": "shadowops-backend",
     "shadowopsEnabled": true
   }
   ```

2. **Host Agent Check**:
   ```bash
   curl -s http://localhost:8081/containers | jq
   ```

3. **Create a Test Workspace**:
   ```bash
   curl -s -X POST http://localhost:8080/api/v1/workspaces \
     -H "Content-Type: application/json" \
     -d '{"name": "Verification Workspace", "targetApp": "google_ads"}' | jq
   ```

4. **Verify WebSocket Live Stream**:
   Open `http://<SERVER_IP>:8080/workspaces/<WORKSPACE_ID>` in Chrome/Firefox and observe the live canvas streaming browser frames.
