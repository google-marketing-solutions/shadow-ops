FROM golang:1.24-bookworm AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o agent .

FROM mcr.microsoft.com/playwright:v1.51.0-noble
WORKDIR /app

RUN mkdir -p /root/.cache/ms-playwright-go/1.50.1 && \
    cd /root/.cache/ms-playwright-go/1.50.1 && \
    npm install --silent playwright@1.50.1 && \
    mv node_modules/playwright package && \
    ln -s $(which node) /root/.cache/ms-playwright-go/1.50.1/node && \
    /root/.cache/ms-playwright-go/1.50.1/node /root/.cache/ms-playwright-go/1.50.1/package/cli.js install chromium chrome

RUN apt-get update -qq && \
    apt-get install -y -qq python3-pip xvfb && \
    pip install -q --break-system-packages playwright==1.51.0 && \
    rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/agent .
COPY scripts ./scripts

ENV AGENT_PORT=8765 \
    BROWSER_HEADLESS=false \
    PLAYWRIGHT_CHANNEL=chrome \
    REAL_AGENT=true

EXPOSE 8765

CMD ["sh", "-lc", "xvfb-run -a -s '-screen 0 1280x800x24' ./agent --port ${AGENT_PORT:-8765} --workspace-id \"${WORKSPACE_ID:-ws_local}\""]
