# Copyright 2026 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Stage 1: Build Angular Frontend
FROM node:20-alpine AS frontend-builder
WORKDIR /app/frontend

COPY frontend/package*.json ./
RUN npm ci --silent

COPY frontend/ ./
RUN npm run build

# Stage 2: Build Go Backend
FROM golang:1.24-bookworm AS backend-builder
WORKDIR /app/backend

ENV GOTOOLCHAIN=auto

RUN apt-get update -qq && \
    apt-get install -y -qq --no-install-recommends gcc libc6-dev && \
    rm -rf /var/lib/apt/lists/*

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -o /app/shadowops-backend .

# Stage 3: Minimal Production Runtime
FROM debian:bookworm-slim

RUN apt-get update -qq && \
    apt-get install -y -qq --no-install-recommends ca-certificates curl tzdata && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app

RUN mkdir -p /app/backend/default-playwright-scripts

COPY --from=backend-builder /app/shadowops-backend /app/shadowops-backend
COPY --from=frontend-builder /app/frontend/dist/frontend/browser /app/public
COPY backend/default-playwright-scripts* /app/backend/default-playwright-scripts/

ENV PORT=8080 \
    BIND_ADDR=:8080 \
    DATA_DIR=/app/data \
    STATIC_DIR=/app/public \
    HOST_AGENT_URL=http://127.0.0.1:8081 \
    WORKSPACE_IMAGE_NAME=shadowops-workspace-agent:dev \
    SHADOWOPS_ENABLED=true

EXPOSE 8080

VOLUME ["/app/data"]

CMD ["/app/shadowops-backend"]
