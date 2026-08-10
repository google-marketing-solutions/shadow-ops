# ShadowOps Platform Deployment & Configuration Guide (`deployment.md`)

This guide provides end-to-end instructions for deploying, configuring, and running **ShadowOps Phase 1** across **Standalone Servers** (Linux VMs, DigitalOcean Droplets, Bare Metal, Local PCs) and **Google Cloud Platform** (Compute Engine VM, Identity-Aware Proxy (IAP) Security).

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

#### 2. Google Cloud Platform (GCP) Compute Engine + IAP Architecture

```
                       ┌────────────────────────┐
                       │   Client Browser (UI)  │
                       └───────────┬────────────┘
                                   │
                      HTTPS / WSS  │  :443 (GCP HTTPS Load Balancer + Managed SSL)
                      or IAP Tunnel│  or gcloud start-iap-tunnel :8080
                                   ▼
                     ┌───────────────────────────┐
                     │ Google Cloud IAP / LB     │
                     │ (Google SSO & IAM Auth)   │
                     └─────────────┬─────────────┘
                                   │
                                   ▼
                     ┌───────────────────────────┐
                     │ Compute Engine (GCE) VM   │
                     │  (shadowops-server)       │
                     │                           │
                     │  ┌─────────────────────┐  │
                     │  │ Backend & Web UI    │  │
                     │  │ (Docker :8080)      │  │
                     │  └──────────┬──────────┘  │
                     │             │             │
                     │  ┌──────────▼──────────┐  │
                     │  │ Host Agent (:8081)  │  │
                     │  └──────────┬──────────┘  │
                     │             │ Launches    │
                     │  ┌──────────▼──────────┐  │
                     │  │ Workspace Container │  │
                     │  │ (Playwright :8765+) │  │
                     │  └─────────────────────┘  │
                     └───────────────────────────┘
```

### HTTP vs. HTTPS Protocol Resolution Rules

| Communication Channel | Supported Protocols | Configuration Variable | Resolution Behavior |
| :--- | :--- | :--- | :--- |
| **Browser $\rightarrow$ Backend** | `http://` or `https://`<br>`ws://` or `wss://` | `BIND_ADDR` / `PORT` | Auto-detected by browser. If accessed via `https://`, Angular client automatically opens WebSocket over secure `wss://`. |
| **Backend $\rightarrow$ Host Agent** | `http://` or `https://` | `HOST_AGENT_URL`<br>`HOST_AGENT_ADDR`<br>`HOST_AGENT_IP` | • `HOST_AGENT_URL` preserves full scheme (e.g. `https://host-agent.internal:8443`).<br>• `HOST_AGENT_ADDR` without scheme defaults to `http://<addr>`.<br>• Go HTTP client verifies TLS certificates with system CAs. |
| **Backend $\rightarrow$ Workspace Agent** | `http://` and `ws://` | *(Automatic)* | Automatically routed to the local allocated container port (`http://127.0.0.1:<port>`). No manual configuration required. |
| **Backend $\rightarrow$ Gemini AI** | `https://` (TLS) | `GEMINI_API_ENDPOINT` | Encrypted HTTPS REST calls to Google AI (`generativelanguage.googleapis.com`) with API key or ADC OAuth tokens. |

| Component | Standalone / VM Mode | GCP Compute Engine (GCE) Mode |
| :--- | :--- | :--- |
| **Backend** | Go binary / Docker container (:8080) | Docker container (:8080) on GCE VM |
| **Frontend UI** | Served directly by Backend or ng serve (:4200) | Integrated into Backend container bundle |
| **Host Agent** | Node.js process (:8081) managing Docker socket | Node.js process (:8081) managing Docker socket |
| **Workspace Agent** | Local Docker containers managed by Host Agent | Docker containers managed by Host Agent |
| **Security & Auth** | Local network / basic firewall | Google Cloud Identity-Aware Proxy (IAP) & IAM |

---

## 2. Environment Variables Reference

All services support environment variable configuration. Copy [`.env.example`](./.env.example) to `.env` to customize settings.

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
> **Workspace Agent is Zero-Configuration**: The Workspace Agent container ([workspace-agent/Dockerfile](./workspace-agent/Dockerfile)) is completely self-contained with bundled Chrome, Playwright, Xvfb, and its internal server. It requires **no manual environment variables or custom configuration**; the platform backend and orchestrator manage all workspace sessions automatically.

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

## 4. Google Cloud Platform (GCP) Compute Engine (GCE) Deployment

Deploy the complete Docker Compose stack onto a single Google Compute Engine (GCE) VM with attached persistent SSD.

### 1. Create Compute Engine VM
```bash
export PROJECT_ID="my-gcp-project"
export ZONE="us-central1-a"

gcloud config set project $PROJECT_ID

# Create GCE instance with Ubuntu 24.04 LTS
gcloud compute instances create shadowops-vm \
  --zone=$ZONE \
  --machine-type="e2-standard-4" \
  --image-family="ubuntu-2404-lts-amd64" \
  --image-project="ubuntu-os-cloud" \
  --boot-disk-size="50GB" \
  --tags="http-server,https-server,shadowops" \
  --scopes="cloud-platform"

# Allow firewall traffic on port 8080 (or restrict using IAP in Section 5)
gcloud compute firewall-rules create allow-shadowops-8080 \
  --allow=tcp:8080 \
  --target-tags="http-server"
```

### 2. Launch Services via Docker Compose
SSH into the instance and run:
```bash
gcloud compute ssh shadowops-vm --zone=$ZONE

# Install Docker & Docker Compose plugin
sudo apt-get update
sudo apt-get install -y docker.io docker-compose-v2
sudo usermod -aG docker $USER
newgrp docker

# Clone repository and launch stack
git clone <repository_url> shadow-ops
cd shadow-ops
docker compose up -d --build
```

---

## 5. Google Cloud Security & Identity-Aware Proxy (IAP)

To protect your ShadowOps deployment so only authorized Google accounts (e.g. `user@yourdomain.com`) can access the dashboard, Google Cloud provides two IAP integration patterns:

---

### Method A: Google Cloud IAP TCP Tunneling (Recommended for Private VMs)

IAP TCP Tunneling provides zero-public-exposure security. The VM's public internet firewall is disabled completely; only authenticated users who hold IAM access in the Google Cloud project can open an encrypted tunnel to the web dashboard.

```
┌──────────────────────────┐    gcloud start-iap-tunnel    ┌──────────────────────────┐     Private Port :8080     ┌──────────────────────────┐
│  Developer Browser       ├──────────────────────────────►│  Google Cloud IAP Proxy  ├───────────────────────────►│  Compute Engine VM     │
│  http://localhost:8080   │   (Auth: user@yourdomain.com) │  (CIDR: 35.235.240.0/20) │    (No Public IP Access) │  (shadowops-server)      │
└──────────────────────────┘                               └──────────────────────────┘                            └──────────────────────────┘
```

#### Step-by-Step Setup:

1. **Create the IAP Firewall Ingress Rule**:
   Allow traffic only from Google's secure IAP forwarders (`35.235.240.0/20`):
   ```bash
   gcloud compute firewall-rules create allow-iap-proxy \
     --project=$PROJECT_ID \
     --direction=INGRESS \
     --priority=1000 \
     --network=default \
     --action=ALLOW \
     --rules=tcp:8080,tcp:22 \
     --source-ranges=35.235.240.0/20 \
     --target-tags=shadowops
   ```

2. **Disable or Delete Any Public Ingress Firewall Rules**:
   ```bash
   # Remove open 0.0.0.0/0 firewall rules
   gcloud compute firewall-rules delete allow-shadowops-8080 --project=$PROJECT_ID
   ```

3. **Grant IAM Permissions**:
   Grant the user the IAP tunnel accessor role:
   ```bash
   gcloud projects add-iam-policy-binding $PROJECT_ID \
     --member="user:user@yourdomain.com" \
     --role="roles/iap.tunnelResourceAccessor"
   ```

4. **Connect to the Dashboard**:
   Run the tunnel command in your terminal:
   ```bash
   gcloud compute start-iap-tunnel shadowops-server 8080 \
     --local-host-port=localhost:8080 \
     --zone=us-central1-a \
     --project=$PROJECT_ID
   ```
   Open **[http://localhost:8080](http://localhost:8080)** in your browser.

---

### Method B: Browser-Based IAP via External HTTPS Load Balancer

Browser-based IAP redirects any user visiting your public domain to a **"Sign in with Google"** OAuth screen before forwarding authenticated requests to the backend.

```
┌──────────────────────────┐      1. HTTPS Request         ┌──────────────────────────┐
│  Client Browser          ├──────────────────────────────►│  Cloud Load Balancer     │
│  https://shadowops.domain│                               │  + Identity-Aware Proxy  │
└──────────────────────────┘                               └────────────┬─────────────┘
             ▲                                                          │
             │ 2. Redirects to "Sign in with Google"                    │ 3. Forwards if user has
             │    (Enforces identity: user@yourdomain.com)              │    roles/iap.httpsResourceAccessor
             ▼                                                          ▼
┌──────────────────────────┐                               ┌──────────────────────────┐
│ Google Accounts / OAuth  │                               │  Unmanaged Instance Group│
│ (Cloud Identity / IdP)   │                               │  (shadowops-server:8080) │
└──────────────────────────┘                               └──────────────────────────┘
```

#### Step-by-Step Setup:

1. **Enable IAP and Resource Manager APIs**:
   ```bash
   gcloud services enable iap.googleapis.com cloudresourcemanager.googleapis.com --project=$PROJECT_ID
   ```

2. **Create OAuth Consent Brand & Credentials**:
   ```bash
   # Create OAuth Brand
   gcloud iap oauth-brands create \
     --application_title="ShadowOps" \
     --support_email="admin@yourdomain.com" \
     --project=$PROJECT_ID

   # Create IAP OAuth Client
   gcloud iap oauth-clients create projects/$PROJECT_NUMBER/brands/$PROJECT_NUMBER \
     --display_name="ShadowOps-IAP-Client" \
     --project=$PROJECT_ID
   ```

3. **Put the VM into an Unmanaged Instance Group**:
   ```bash
   gcloud compute instance-groups unmanaged create shadowops-ig \
     --project=$PROJECT_ID \
     --zone=us-central1-a

   gcloud compute instance-groups unmanaged add-instances shadowops-ig \
     --project=$PROJECT_ID \
     --zone=us-central1-a \
     --instances=shadowops-server

   gcloud compute instance-groups unmanaged set-named-ports shadowops-ig \
     --project=$PROJECT_ID \
     --zone=us-central1-a \
     --named-ports=http:8080
   ```

4. **Create Health Check & IAP-Secured Backend Service**:
   ```bash
   # Create Health Check
   gcloud compute health-checks create http shadowops-hc \
     --project=$PROJECT_ID \
     --port=8080 \
     --request-path=/api/v1/health

   # Create Backend Service with IAP Enabled
   gcloud compute backend-services create shadowops-backend-service \
     --project=$PROJECT_ID \
     --protocol=HTTP \
     --port-name=http \
     --health-checks=shadowops-hc \
     --global \
     --iap=enabled,oauth2-client-id=$OAUTH_CLIENT_ID,oauth2-client-secret=$OAUTH_CLIENT_SECRET

   # Add Instance Group to Backend Service
   gcloud compute backend-services add-backend shadowops-backend-service \
     --project=$PROJECT_ID \
     --global \
     --instance-group=shadowops-ig \
     --instance-group-zone=us-central1-a \
     --balancing-mode=UTILIZATION \
     --max-utilization=0.8
   ```

5. **Grant IAP Web Access to Specified Google Accounts**:
   ```bash
   gcloud iap web add-iam-policy-binding \
     --resource-type=backend-services \
     --service=shadowops-backend-service \
     --member="user:user@yourdomain.com" \
     --role="roles/iap.httpsResourceAccessor" \
     --project=$PROJECT_ID
   ```

6. **Create Load Balancer Routing & HTTPS Frontend**:
   ```bash
   # Reserve Global Static IP
   gcloud compute addresses create shadowops-lb-ip --global --project=$PROJECT_ID

   # Create URL Map
   gcloud compute url-maps create shadowops-url-map \
     --default-service=shadowops-backend-service \
     --global \
     --project=$PROJECT_ID

   # Create Google-Managed SSL Certificate for your domain
   gcloud compute ssl-certificates create shadowops-ssl-cert \
     --domains="shadowops.yourdomain.com" \
     --global \
     --project=$PROJECT_ID

   # Create Target HTTPS Proxy
   gcloud compute target-https-proxies create shadowops-https-proxy \
     --url-map=shadowops-url-map \
     --ssl-certificates=shadowops-ssl-cert \
     --global \
     --project=$PROJECT_ID

   # Create Global Forwarding Rule on Port 443
   gcloud compute forwarding-rules create shadowops-https-rule \
     --global \
     --target-https-proxy=shadowops-https-proxy \
     --ports=443 \
     --address=shadowops-lb-ip \
     --project=$PROJECT_ID
   ```

> [!NOTE]
> **Organization Policy Constraint**: If your Google Cloud project enforces `constraints/compute.restrictLoadBalancerCreationForTypes` restricting external public load balancers, use **Method A (IAP TCP Tunneling)** or an in-cluster OAuth proxy (e.g. `oauth2-proxy`) on the VM.

---

## 6. Verification & Health Checks

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
   Open `http://localhost:8080/workspaces/<WORKSPACE_ID>` (or `http://<SERVER_IP>:8080/workspaces/<WORKSPACE_ID>`) in Chrome/Firefox and observe the live canvas streaming browser frames.
