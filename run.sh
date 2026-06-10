#!/bin/bash

# Function to handle cleanup on exit
cleanup() {
    echo "Stopping host agent, frontend and backend..."
    kill $HOST_AGENT_PID 2>/dev/null
    kill $BACKEND_PID 2>/dev/null
    kill $FRONTEND_PID 2>/dev/null
    exit
}

# Trap SIGINT and SIGTERM
trap cleanup SIGINT SIGTERM

# Get the script's directory
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

echo "Starting host agent..."
cd "$SCRIPT_DIR/host-agent"
npm install
node index.js &
HOST_AGENT_PID=$!

echo "Starting backend..."
cd "$SCRIPT_DIR/backend"
go run . &
BACKEND_PID=$!

echo "Starting frontend..."
cd "$SCRIPT_DIR/frontend"
npm start &
FRONTEND_PID=$!

echo "Host Agent PID: $HOST_AGENT_PID"
echo "Backend PID: $BACKEND_PID"
echo "Frontend PID: $FRONTEND_PID"
echo "Press Ctrl+C to stop all."

# Wait for all processes
wait $HOST_AGENT_PID $BACKEND_PID $FRONTEND_PID
