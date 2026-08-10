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
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// ClientConn wraps a WebSocket connection with synchronized writes.
type ClientConn struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

// WriteJSON sends a JSON message over the WebSocket safely.
func (c *ClientConn) WriteJSON(v interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.WriteJSON(v)
}

// AgentServer serves HTTP REST and WebSocket endpoints for the workspace agent.
type AgentServer struct {
	stateMgr   *StateManager
	engine     BrowserEngine
	clientsMu  sync.RWMutex
	clients    map[*ClientConn]bool
	upgrader   websocket.Upgrader
	shutdownCh chan struct{}
	frameStop  chan struct{}
}

// NewAgentServer creates a new AgentServer instance.
func NewAgentServer(mgr *StateManager, engine BrowserEngine) *AgentServer {
	return &AgentServer{
		stateMgr:   mgr,
		engine:     engine,
		clients:    make(map[*ClientConn]bool),
		shutdownCh: make(chan struct{}),
		frameStop:  make(chan struct{}),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
			EnableCompression: true,
		},
	}
}

// SetupRoutes registers all HTTP handlers onto the provided ServeMux.
func (s *AgentServer) SetupRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /status", s.handleStatus)
	mux.HandleFunc("POST /navigate", s.handleNavigate)
	mux.HandleFunc("POST /input", s.handleInput)
	mux.HandleFunc("POST /shadow/start", s.handleShadowStart)
	mux.HandleFunc("POST /shadow/end", s.handleShadowEnd)
	mux.HandleFunc("GET /shadow/actions", s.handleShadowActions)
	mux.HandleFunc("POST /kill", s.handleKill)
	mux.HandleFunc("POST /script/run", s.handleScriptRun)
	mux.HandleFunc("POST /script/cancel", s.handleScriptCancel)
	mux.HandleFunc("GET /script/status", s.handleScriptStatus)
	mux.HandleFunc("GET /events", s.handleEventsWS)
}

// StartFrameLoop begins periodic frame rendering and broadcast.
func (s *AgentServer) StartFrameLoop() {
	go func() {
		ticker := time.NewTicker(400 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-s.frameStop:
				return
			case <-ticker.C:
				if s.numClients() == 0 {
					continue
				}
				frame, err := s.engine.RenderFrame(context.Background())
				if err == nil && frame != nil {
					s.Broadcast(frame)
				}
			}
		}
	}()
}

// StopFrameLoop stops the background frame rendering loop.
func (s *AgentServer) StopFrameLoop() {
	select {
	case <-s.frameStop:
	default:
		close(s.frameStop)
	}
}

func (s *AgentServer) numClients() int {
	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()
	return len(s.clients)
}

// Broadcast sends a message to all connected WebSocket clients.
func (s *AgentServer) Broadcast(v interface{}) {
	s.clientsMu.RLock()
	conns := make([]*ClientConn, 0, len(s.clients))
	for c := range s.clients {
		conns = append(conns, c)
	}
	s.clientsMu.RUnlock()

	for _, c := range conns {
		if err := c.WriteJSON(v); err != nil {
			s.clientsMu.Lock()
			delete(s.clients, c)
			s.clientsMu.Unlock()
			_ = c.conn.Close()
		}
	}
}

func (s *AgentServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *AgentServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	st := s.stateMgr.GetState()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":         st.Status,
		"recordingState": st.RecordingState,
		"currentUrl":     st.CurrentURL,
		"scriptStatus":   st.ScriptRun.Status,
		"scriptRunId":    st.ScriptRun.RunID,
	})
}

func (s *AgentServer) handleNavigate(w http.ResponseWriter, r *http.Request) {
	var in NavigateInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.URL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "url required"})
		return
	}

	out, err := s.engine.Navigate(r.Context(), in)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}

	st := s.stateMgr.GetState()
	s.Broadcast(map[string]interface{}{
		"type":           "workspace.status",
		"status":         st.Status,
		"recordingState": st.RecordingState,
		"currentUrl":     st.CurrentURL,
	})

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":         true,
		"currentUrl": out.CurrentURL,
	})
}

func (s *AgentServer) handleInput(w http.ResponseWriter, r *http.Request) {
	var payload InputPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if err := s.engine.HandleInput(r.Context(), payload); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *AgentServer) handleShadowStart(w http.ResponseWriter, r *http.Request) {
	if err := s.engine.StartShadow(r.Context()); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	st := s.stateMgr.GetState()
	s.Broadcast(map[string]interface{}{
		"type":           "workspace.status",
		"status":         st.Status,
		"recordingState": st.RecordingState,
		"currentUrl":     st.CurrentURL,
	})
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":             true,
		"recordingState": "on",
		"startedAt":      st.RecordingStartedAt,
	})
}

func (s *AgentServer) handleShadowEnd(w http.ResponseWriter, r *http.Request) {
	steps, err := s.engine.EndShadow(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	st := s.stateMgr.GetState()
	s.Broadcast(map[string]interface{}{
		"type":           "workspace.status",
		"status":         st.Status,
		"recordingState": st.RecordingState,
		"currentUrl":     st.CurrentURL,
	})
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":             true,
		"recordingState": "off",
		"steps":          steps,
	})
}

func (s *AgentServer) handleShadowActions(w http.ResponseWriter, r *http.Request) {
	st := s.stateMgr.GetState()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"workspaceId": st.WorkspaceID,
		"steps":       st.Steps,
	})
}

func (s *AgentServer) handleKill(w http.ResponseWriter, r *http.Request) {
	s.stateMgr.UpdateState(func(st *AgentState) {
		st.Status = "killed"
		st.RecordingState = "off"
	})
	st := s.stateMgr.GetState()
	s.Broadcast(map[string]interface{}{
		"type":           "workspace.status",
		"status":         st.Status,
		"recordingState": st.RecordingState,
		"currentUrl":     st.CurrentURL,
	})

	go func() {
		time.Sleep(200 * time.Millisecond)
		select {
		case <-s.shutdownCh:
		default:
			close(s.shutdownCh)
		}
	}()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":     true,
		"status": "killed",
	})
}

func (s *AgentServer) handleScriptRun(w http.ResponseWriter, r *http.Request) {
	var in ScriptRunInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Script == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "script required"})
		return
	}

	st := s.stateMgr.GetState()
	if st.ScriptRun.Status == "running" {
		writeJSON(w, http.StatusConflict, map[string]interface{}{
			"error":  "script already running",
			"runId":  st.ScriptRun.RunID,
			"status": st.ScriptRun.Status,
		})
		return
	}

	runID := fmt.Sprintf("scr_%d", time.Now().UnixNano())
	in.RunID = runID
	if in.Filename == "" {
		in.Filename = "script.py"
	}

	s.stateMgr.UpdateState(func(s *AgentState) {
		s.ScriptRun = ScriptRunState{
			RunID:     runID,
			Status:    "running",
			Filename:  in.Filename,
			Inputs:    in.Inputs,
			StartedAt: NowISO(),
			Logs:      make([]ScriptLogEntry, 0),
		}
	})

	s.Broadcast(map[string]interface{}{
		"type":     "script.status",
		"runId":    runID,
		"status":   "running",
		"filename": in.Filename,
	})

	go func() {
		out, err := s.engine.RunScript(context.Background(), in, func(entry ScriptLogEntry) {
			s.stateMgr.UpdateState(func(st *AgentState) {
				st.ScriptRun.Logs = append(st.ScriptRun.Logs, entry)
			})
			s.Broadcast(map[string]interface{}{
				"type":    "script.log",
				"runId":   runID,
				"ts":      entry.Timestamp,
				"level":   entry.Level,
				"message": entry.Message,
			})
		})

		s.stateMgr.UpdateState(func(st *AgentState) {
			if err != nil && out.Status == "" {
				st.ScriptRun.Status = "failed"
				st.ScriptRun.Error = err.Error()
			} else {
				st.ScriptRun.Status = out.Status
				st.ScriptRun.Result = out.Result
				st.ScriptRun.Error = out.Error
			}
			st.ScriptRun.FinishedAt = NowISO()
		})

		finishedSt := s.stateMgr.GetState()
		s.Broadcast(map[string]interface{}{
			"type":       "script.result",
			"runId":      runID,
			"status":     finishedSt.ScriptRun.Status,
			"result":     finishedSt.ScriptRun.Result,
			"error":      finishedSt.ScriptRun.Error,
			"finishedAt": finishedSt.ScriptRun.FinishedAt,
		})
	}()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":     true,
		"runId":  runID,
		"status": "running",
	})
}

func (s *AgentServer) handleScriptCancel(w http.ResponseWriter, r *http.Request) {
	st := s.stateMgr.GetState()
	if st.ScriptRun.Status != "running" {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "no running script"})
		return
	}

	runID := st.ScriptRun.RunID
	s.stateMgr.UpdateState(func(s *AgentState) {
		s.ScriptRun.Status = "cancelling"
		s.ScriptRun.CancelRequested = true
	})

	_ = s.engine.CancelScript(r.Context(), runID)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":     true,
		"runId":  runID,
		"status": "cancelling",
	})
}

func (s *AgentServer) handleScriptStatus(w http.ResponseWriter, r *http.Request) {
	st := s.stateMgr.GetState()
	writeJSON(w, http.StatusOK, st.ScriptRun)
}

func (s *AgentServer) handleEventsWS(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("failed to upgrade websocket: %v", err)
		return
	}
	client := &ClientConn{conn: conn}

	s.clientsMu.Lock()
	s.clients[client] = true
	s.clientsMu.Unlock()

	defer func() {
		s.clientsMu.Lock()
		delete(s.clients, client)
		s.clientsMu.Unlock()
		_ = conn.Close()
	}()

	st := s.stateMgr.GetState()
	_ = client.WriteJSON(map[string]interface{}{
		"type":           "workspace.status",
		"status":         st.Status,
		"recordingState": st.RecordingState,
		"currentUrl":     st.CurrentURL,
	})
	for _, step := range st.Steps {
		_ = client.WriteJSON(map[string]interface{}{
			"type": "shadow.step",
			"step": step,
		})
	}

	for {
		var msg map[string]interface{}
		if err := conn.ReadJSON(&msg); err != nil {
			break
		}
		s.handleIncomingWSMessage(msg)
	}
}

func (s *AgentServer) handleIncomingWSMessage(msg map[string]interface{}) {
	msgType, _ := msg["type"].(string)
	switch msgType {
	case "shadow.start":
		_ = s.engine.StartShadow(context.Background())
	case "shadow.end":
		_, _ = s.engine.EndShadow(context.Background())
	}
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
