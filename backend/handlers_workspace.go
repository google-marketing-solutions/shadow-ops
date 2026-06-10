package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (s *Server) handleListWorkspaces(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	list := make([]map[string]any, 0, len(s.workspaces))
	for _, ws := range s.workspaces {
		ws.mu.RLock()
		item := map[string]any{
			"workspaceId":    ws.ID,
			"name":           ws.Name,
			"status":         ws.Status,
			"recordingState": ws.RecordingState,
			"currentUrl":     ws.CurrentURL,
			"createdAt":      ws.CreatedAt,
			"updatedAt":      ws.UpdatedAt,
		}
		ws.mu.RUnlock()
		list = append(list, item)
	}
	s.mu.RUnlock()
	writeJSON(w, http.StatusOK, map[string]any{"items": list})
}

func (s *Server) handleCreateWorkspace(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TargetApp string `json:"targetApp"`
		Name      string `json:"name"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "WORKSPACE_NAME_REQUIRED", "Workspace name is required. Please choose a name.")
		return
	}
	if req.TargetApp == "" {
		req.TargetApp = "google_ads"
	}

	s.mu.RLock()
	for _, ws := range s.workspaces {
		ws.mu.RLock()
		if strings.EqualFold(ws.Name, req.Name) && ws.Status != "killed" {
			ws.mu.RUnlock()
			s.mu.RUnlock()
			writeError(w, http.StatusBadRequest, "REPETITIVE_WORKSPACE_NAME", fmt.Sprintf("Workspace name '%s' already exists. Please choose a unique name.", req.Name))
			return
		}
		ws.mu.RUnlock()
	}
	s.mu.RUnlock()

	id := "ws_" + shortID()
	port, err := randomPort()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "PORT_ALLOC_FAILED", err.Error())
		return
	}

	if err := s.ensureWorkspaceImage(); err != nil {
		writeError(w, http.StatusInternalServerError, "DOCKER_IMAGE_BUILD_FAILED", err.Error())
		return
	}

	containerID, err := s.runWorkspaceContainer(id, port)
	if err != nil {
		writeError(w, http.StatusBadGateway, "WORKSPACE_CONTAINER_START_FAILED", err.Error())
		return
	}

	ws := &Workspace{
		ID:                id,
		Name:              req.Name,
		TargetApp:         req.TargetApp,
		Status:            "launching",
		RecordingState:    "off",
		CurrentURL:        "https://accounts.google.com/ServiceLogin/signinchooser",
		CreatedAt:         time.Now().UTC(),
		UpdatedAt:         time.Now().UTC(),
		AgentPort:         port,
		AgentBaseURL:      fmt.Sprintf("http://127.0.0.1:%d", port),
		DockerContainerID: containerID,
		clients:           map[*WSClient]struct{}{},
	}
	log.Printf("workspace %s container=%s port=%d", id, shortContainerID(containerID), port)

	s.mu.Lock()
	s.workspaces[id] = ws
	s.mu.Unlock()

	_, err = s.db.Exec(`INSERT OR REPLACE INTO workspaces (id, name, target_app, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		ws.ID, ws.Name, ws.TargetApp, ws.CreatedAt.Format(time.RFC3339), ws.UpdatedAt.Format(time.RFC3339))
	if err != nil {
		log.Printf("warning: failed inserting workspace %s into sqlite: %v", id, err)
	}

	if err := s.waitForAgentHealth(ws, 30*time.Second); err != nil {
		_ = s.removeWorkspaceContainer(ws)
		ws.mu.Lock()
		ws.Status = "error"
		ws.UpdatedAt = time.Now().UTC()
		ws.mu.Unlock()
		writeError(w, http.StatusBadGateway, "AGENT_HEALTH_TIMEOUT", err.Error())
		return
	}

	// prime status from agent and start event pump
	if err := s.refreshWorkspaceStatus(ws); err != nil {
		log.Printf("workspace %s status refresh warning: %v", id, err)
	}
	go s.consumeAgentEvents(ws)

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "workspaceId": id, "name": req.Name})
}

func (s *Server) handleRenameWorkspace(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.getWorkspace(r.PathValue("workspaceId"))
	if !ok {
		writeError(w, http.StatusNotFound, "WORKSPACE_NOT_FOUND", "workspace not found")
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "WORKSPACE_NAME_REQUIRED", "Workspace name is required")
		return
	}

	s.mu.RLock()
	for _, existing := range s.workspaces {
		existing.mu.RLock()
		if existing.ID != ws.ID && strings.EqualFold(existing.Name, req.Name) && existing.Status != "killed" {
			existing.mu.RUnlock()
			s.mu.RUnlock()
			writeError(w, http.StatusBadRequest, "REPETITIVE_WORKSPACE_NAME", fmt.Sprintf("Workspace name '%s' already exists. Please choose a unique name.", req.Name))
			return
		}
		existing.mu.RUnlock()
	}
	s.mu.RUnlock()

	ws.mu.Lock()
	ws.Name = req.Name
	ws.UpdatedAt = time.Now().UTC()
	ws.mu.Unlock()

	_, err := s.db.Exec(`UPDATE workspaces SET name = ?, updated_at = ? WHERE id = ?`,
		req.Name, time.Now().UTC().Format(time.RFC3339), ws.ID)
	if err != nil {
		log.Printf("warning: failed updating workspace %s name in sqlite: %v", ws.ID, err)
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "name": req.Name})
}

func (s *Server) handleGetWorkspace(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.getWorkspace(r.PathValue("workspaceId"))
	if !ok {
		writeError(w, http.StatusNotFound, "WORKSPACE_NOT_FOUND", "workspace not found")
		return
	}
	if err := s.refreshWorkspaceStatus(ws); err != nil {
		log.Printf("workspace refresh error: %v", err)
	}
	ws.mu.RLock()
	resp := map[string]any{
		"workspaceId":    ws.ID,
		"name":           ws.Name,
		"status":         ws.Status,
		"recordingState": ws.RecordingState,
		"currentUrl":     ws.CurrentURL,
		"createdAt":      ws.CreatedAt,
	}
	ws.mu.RUnlock()
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleNavigateWorkspace(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.getWorkspace(r.PathValue("workspaceId"))
	if !ok {
		writeError(w, http.StatusNotFound, "WORKSPACE_NOT_FOUND", "workspace not found")
		return
	}
	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.URL) == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "url is required")
		return
	}
	var resp map[string]any
	if err := s.agentPostJSON(ws, "/navigate", map[string]any{"url": req.URL}, &resp); err != nil {
		writeError(w, http.StatusBadGateway, "AGENT_NAVIGATE_FAILED", err.Error())
		return
	}
	_ = s.refreshWorkspaceStatus(ws)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleKillWorkspace(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.getWorkspace(r.PathValue("workspaceId"))
	if !ok {
		writeError(w, http.StatusNotFound, "WORKSPACE_NOT_FOUND", "workspace not found")
		return
	}

	_ = s.agentPostJSON(ws, "/kill", map[string]any{}, nil)
	_ = s.removeWorkspaceContainer(ws)

	ws.mu.Lock()
	ws.Status = "killed"
	ws.RecordingState = "off"
	now := time.Now().UTC()
	ws.EndedAt = &now
	ws.UpdatedAt = now
	ws.mu.Unlock()
	s.broadcastEvent(ws, map[string]any{"type": "workspace.status", "status": "killed", "recordingState": "off", "currentUrl": ws.CurrentURL})

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "status": "killed"})
}

func (s *Server) handleShadowStart(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.getWorkspace(r.PathValue("workspaceId"))
	if !ok {
		writeError(w, http.StatusNotFound, "WORKSPACE_NOT_FOUND", "workspace not found")
		return
	}
	var resp map[string]any
	if err := s.agentPostJSON(ws, "/shadow/start", map[string]any{}, &resp); err != nil {
		writeError(w, http.StatusBadGateway, "SHADOW_START_FAILED", err.Error())
		return
	}

	ws.mu.Lock()
	ws.RecordingState = "on"
	ws.Status = "recording"
	ws.captureSessionID = fmt.Sprintf("cap_%s", shortID())
	ws.steps = nil
	ws.UpdatedAt = time.Now().UTC()
	captureID := ws.captureSessionID
	ws.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":               true,
		"recordingState":   "on",
		"startedAt":        time.Now().UTC(),
		"captureSessionId": captureID,
	})
}

func (s *Server) handleShadowEnd(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.getWorkspace(r.PathValue("workspaceId"))
	if !ok {
		writeError(w, http.StatusNotFound, "WORKSPACE_NOT_FOUND", "workspace not found")
		return
	}
	var resp struct {
		Ok             bool             `json:"ok"`
		RecordingState string           `json:"recordingState"`
		Steps          []map[string]any `json:"steps"`
	}
	if err := s.agentPostJSON(ws, "/shadow/end", map[string]any{}, &resp); err != nil {
		writeError(w, http.StatusBadGateway, "SHADOW_END_FAILED", err.Error())
		return
	}

	ws.mu.Lock()
	ws.RecordingState = "off"
	ws.Status = "active"
	ws.steps = deepCopySteps(resp.Steps)
	ws.UpdatedAt = time.Now().UTC()
	captureID := ws.captureSessionID
	if captureID == "" {
		captureID = fmt.Sprintf("cap_%s", shortID())
		ws.captureSessionID = captureID
	}
	steps := deepCopySteps(ws.steps)
	ws.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":               true,
		"recordingState":   "off",
		"captureSessionId": captureID,
		"steps":            steps,
	})
}

func (s *Server) handleShadowActions(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.getWorkspace(r.PathValue("workspaceId"))
	if !ok {
		writeError(w, http.StatusNotFound, "WORKSPACE_NOT_FOUND", "workspace not found")
		return
	}
	ws.mu.RLock()
	steps := deepCopySteps(ws.steps)
	captureID := ws.captureSessionID
	ws.mu.RUnlock()
	writeJSON(w, http.StatusOK, map[string]any{"captureSessionId": captureID, "steps": steps})
}

func (s *Server) handleScriptRun(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.getWorkspace(r.PathValue("workspaceId"))
	if !ok {
		writeError(w, http.StatusNotFound, "WORKSPACE_NOT_FOUND", "workspace not found")
		return
	}
	var req struct {
		Filename  string `json:"filename"`
		Script    string `json:"script"`
		TargetURL string `json:"targetUrl"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON")
		return
	}
	if strings.TrimSpace(req.Script) == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "script is required")
		return
	}
	var resp map[string]any
	if err := s.agentPostJSON(ws, "/script/run", map[string]any{
		"filename":  req.Filename,
		"script":    req.Script,
		"targetUrl": req.TargetURL,
	}, &resp); err != nil {
		writeError(w, http.StatusBadGateway, "SCRIPT_RUN_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleScriptStatus(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.getWorkspace(r.PathValue("workspaceId"))
	if !ok {
		writeError(w, http.StatusNotFound, "WORKSPACE_NOT_FOUND", "workspace not found")
		return
	}
	var resp map[string]any
	if err := s.agentGetJSON(ws, "/script/status", &resp); err != nil {
		writeError(w, http.StatusBadGateway, "SCRIPT_STATUS_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleScriptCancel(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.getWorkspace(r.PathValue("workspaceId"))
	if !ok {
		writeError(w, http.StatusNotFound, "WORKSPACE_NOT_FOUND", "workspace not found")
		return
	}
	var resp map[string]any
	if err := s.agentPostJSON(ws, "/script/cancel", map[string]any{}, &resp); err != nil {
		writeError(w, http.StatusBadGateway, "SCRIPT_CANCEL_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleListDefaultScripts(w http.ResponseWriter, r *http.Request) {
	dir := filepath.Join(s.repoRoot, "backend", "default-playwright-scripts")
	entries, err := os.ReadDir(dir)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"items": []map[string]any{}})
		return
	}

	var items []map[string]any
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".py") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		items = append(items, map[string]any{
			"filename": entry.Name(),
			"source":   string(content),
		})
	}

	if items == nil {
		items = []map[string]any{}
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

