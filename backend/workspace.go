package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Workspace struct {
	mu sync.RWMutex

	ID             string     `json:"workspaceId"`
	Name           string     `json:"name"`
	TargetApp      string     `json:"targetApp,omitempty"`
	APIKey         string     `json:"apiKey,omitempty"`
	Status         string     `json:"status"`
	RecordingState string     `json:"recordingState"`
	CurrentURL     string     `json:"currentUrl,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
	EndedAt        *time.Time `json:"endedAt,omitempty"`

	AgentPort         int
	AgentBaseURL      string
	DockerContainerID string

	captureSessionID string
	steps            []map[string]any

	clients map[*WSClient]struct{}
}

func (s *Server) getWorkspace(id string) (*Workspace, bool) {
	s.mu.RLock()
	ws, ok := s.workspaces[id]
	s.mu.RUnlock()
	return ws, ok
}

func (s *Server) waitForAgentHealth(ws *Workspace, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if err := s.agentGetJSON(ws, "/health", nil); err == nil {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("agent not healthy within %s", timeout)
}

func (s *Server) refreshWorkspaceStatus(ws *Workspace) error {
	var status struct {
		Status         string `json:"status"`
		RecordingState string `json:"recordingState"`
		CurrentURL     string `json:"currentUrl"`
	}
	if err := s.agentGetJSON(ws, "/status", &status); err != nil {
		return err
	}
	ws.mu.Lock()
	if status.Status != "" {
		ws.Status = status.Status
	}
	if status.RecordingState != "" {
		ws.RecordingState = status.RecordingState
	}
	ws.CurrentURL = status.CurrentURL
	ws.UpdatedAt = time.Now().UTC()
	ws.mu.Unlock()
	return nil
}

func (s *Server) agentGetJSON(ws *Workspace, path string, out any) error {
	req, err := http.NewRequest(http.MethodGet, ws.AgentBaseURL+path, nil)
	if err != nil {
		return err
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("agent GET %s status=%d body=%s", path, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (s *Server) agentPostJSON(ws *Workspace, path string, payload any, out any) error {
	buf := new(bytes.Buffer)
	if payload == nil {
		payload = map[string]any{}
	}
	if err := json.NewEncoder(buf).Encode(payload); err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, ws.AgentBaseURL+path, buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("agent POST %s status=%d body=%s", path, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
