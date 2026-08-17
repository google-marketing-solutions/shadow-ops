// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type WSClient struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func (s *Server) handleWorkspaceWS(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.getWorkspace(r.PathValue("workspaceId"))
	if !ok {
		writeError(w, http.StatusNotFound, "WORKSPACE_NOT_FOUND", "workspace not found")
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws upgrade error: %v", err)
		return
	}
	client := &WSClient{conn: conn}
	ws.mu.Lock()
	ws.clients[client] = struct{}{}
	snapshot := map[string]any{
		"type":           "workspace.status",
		"status":         ws.Status,
		"recordingState": ws.RecordingState,
		"currentUrl":     ws.CurrentURL,
	}
	ws.mu.Unlock()

	_ = client.writeJSON(snapshot)

	defer func() {
		ws.mu.Lock()
		delete(ws.clients, client)
		ws.mu.Unlock()
		_ = conn.Close()
	}()

	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("frontend ws read error: %v", err)
			}
			return
		}
		var msg map[string]any
		if err := json.Unmarshal(payload, &msg); err != nil {
			_ = client.writeJSON(map[string]any{"type": "error", "code": "INVALID_WS_MESSAGE", "message": "Invalid JSON"})
			continue
		}
		if msgType, _ := msg["type"].(string); msgType == "navigate" {
			navURL, _ := msg["url"].(string)
			if navURL != "" {
				var resp map[string]any
				if err := s.agentPostJSON(ws, "/navigate", map[string]any{"url": navURL}, &resp); err != nil {
					_ = client.writeJSON(map[string]any{"type": "error", "code": "AGENT_NAVIGATE_FAILED", "message": err.Error()})
				} else {
					_ = s.refreshWorkspaceStatus(ws)
				}
			}
			continue
		}
		go func(inputMsg map[string]any) {
			if err := s.agentPostJSON(ws, "/input", inputMsg, nil); err != nil {
				_ = client.writeJSON(map[string]any{"type": "error", "code": "AGENT_INPUT_FAILED", "message": err.Error()})
			}
		}(msg)
	}
}

func (s *Server) consumeAgentEvents(ws *Workspace) {
	u, _ := url.Parse(ws.AgentBaseURL)
	u.Scheme = "ws"
	u.Path = "/events"

	for {
		ws.mu.RLock()
		status := ws.Status
		ws.mu.RUnlock()
		if status == "killed" {
			return
		}

		conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
		if err != nil {
			log.Printf("agent events dial failed (%s): %v", ws.ID, err)
			s.broadcastEvent(ws, map[string]any{"type": "error", "code": "WORKSPACE_DISCONNECTED", "message": "Workspace agent disconnected"})
			time.Sleep(2 * time.Second)
			continue
		}

		conn.SetReadLimit(1024 * 1024)
		_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		conn.SetPongHandler(func(string) error {
			_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
			return nil
		})

		pingDone := make(chan struct{})
		go func() {
			ticker := time.NewTicker(20 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-pingDone:
					return
				case <-ticker.C:
					_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
					if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
						return
					}
				}
			}
		}()

		for {
			_, payload, err := conn.ReadMessage()
			if err != nil {
				log.Printf("agent events read ended (%s): %v", ws.ID, err)
				s.broadcastEvent(ws, map[string]any{"type": "error", "code": "WORKSPACE_DISCONNECTED", "message": "Workspace agent disconnected"})
				break
			}
			_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
			s.processAgentEvent(ws, payload)
			s.broadcastRaw(ws, payload)
		}

		close(pingDone)
		_ = conn.Close()
		time.Sleep(1 * time.Second)
	}
}

func (s *Server) processAgentEvent(ws *Workspace, payload []byte) {
	var msg map[string]any
	if err := json.Unmarshal(payload, &msg); err != nil {
		return
	}
	typ, _ := msg["type"].(string)
	switch typ {
	case "workspace.status":
		ws.mu.Lock()
		if v, ok := msg["status"].(string); ok && v != "" {
			ws.Status = v
		}
		if v, ok := msg["recordingState"].(string); ok && v != "" {
			ws.RecordingState = v
		}
		if v, ok := msg["currentUrl"].(string); ok {
			ws.CurrentURL = v
		}
		ws.UpdatedAt = time.Now().UTC()
		ws.mu.Unlock()
	case "shadow.step":
		step, ok := msg["step"].(map[string]any)
		if !ok {
			return
		}
		ws.mu.Lock()
		idx := int(numberToFloat(step["stepIndex"]))
		if idx <= 0 {
			idx = len(ws.steps) + 1
			step["stepIndex"] = idx
		}
		if idx-1 < len(ws.steps) {
			ws.steps[idx-1] = deepCopyStep(step)
		} else if idx-1 == len(ws.steps) {
			ws.steps = append(ws.steps, deepCopyStep(step))
		} else {
			for len(ws.steps) < idx-1 {
				ws.steps = append(ws.steps, map[string]any{"stepIndex": len(ws.steps) + 1, "type": "noop"})
			}
			ws.steps = append(ws.steps, deepCopyStep(step))
		}
		ws.mu.Unlock()
	}
}

func (s *Server) broadcastEvent(ws *Workspace, msg map[string]any) {
	payload, _ := json.Marshal(msg)
	s.broadcastRaw(ws, payload)
}

func (s *Server) broadcastRaw(ws *Workspace, payload []byte) {
	ws.mu.RLock()
	clients := make([]*WSClient, 0, len(ws.clients))
	for c := range ws.clients {
		clients = append(clients, c)
	}
	ws.mu.RUnlock()
	for _, c := range clients {
		if err := c.writeRaw(payload); err != nil {
			ws.mu.Lock()
			delete(ws.clients, c)
			ws.mu.Unlock()
			_ = c.conn.Close()
		}
	}
}

func (c *WSClient) writeJSON(v any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_ = c.conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	return c.conn.WriteJSON(v)
}

func (c *WSClient) writeRaw(payload []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_ = c.conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	return c.conn.WriteMessage(websocket.TextMessage, payload)
}
