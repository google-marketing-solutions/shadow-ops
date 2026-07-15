package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
)

func (s *Server) recoverWorkspaces() {
	respBody, statusCode, err := s.callHostAgent(context.Background(), "GET", "/containers", nil)
	if err != nil || statusCode >= 300 {
		log.Printf("Failed to recover workspaces: %v (status %d)", err, statusCode)
		return
	}

	var containers []map[string]any
	if err := json.Unmarshal(respBody, &containers); err != nil {
		log.Printf("Failed to parse containers: %v", err)
		return
	}

	for _, c := range containers {
		names, ok := c["Names"].([]any)
		if !ok || len(names) == 0 {
			continue
		}
		name := names[0].(string)
		if !strings.HasPrefix(name, "/shadowops-") {
			continue
		}

		id := c["Id"].(string)

		// Inspect container to get env vars
		inspectBody, statusCode, err := s.callHostAgent(context.Background(), "GET", "/containers/"+id+"/json", nil)
		if err != nil || statusCode >= 300 {
			continue
		}

		var inspect struct {
			Config struct {
				Env []string `json:"Env"`
			} `json:"Config"`
		}
		if err := json.Unmarshal(inspectBody, &inspect); err != nil {
			continue
		}

		var agentPort int
		var workspaceID string
		for _, env := range inspect.Config.Env {
			if strings.HasPrefix(env, "AGENT_PORT=") {
				agentPort, _ = strconv.Atoi(strings.TrimPrefix(env, "AGENT_PORT="))
			} else if strings.HasPrefix(env, "WORKSPACE_ID=") {
				workspaceID = strings.TrimPrefix(env, "WORKSPACE_ID=")
				workspaceID = strings.ReplaceAll(workspaceID, "-", "_")
			}
		}

		if agentPort == 0 || workspaceID == "" {
			continue
		}

		ws := &Workspace{
			ID:                workspaceID,
			Status:            "active",
			RecordingState:    "off",
			CreatedAt:         time.Now().UTC(),
			UpdatedAt:         time.Now().UTC(),
			AgentPort:         agentPort,
			AgentBaseURL:      fmt.Sprintf("http://127.0.0.1:%d", agentPort),
			DockerContainerID: id,
			clients:           map[*WSClient]struct{}{},
		}

		var recoveredName, recoveredAPIKey string
		err = s.db.QueryRow(`SELECT name, COALESCE(api_key, '') FROM workspaces WHERE id = ?`, workspaceID).Scan(&recoveredName, &recoveredAPIKey)
		if err == nil {
			if recoveredName != "" {
				ws.Name = recoveredName
			} else {
				ws.Name = workspaceID
			}
			if recoveredAPIKey == "" {
				ws.APIKey = "sk_" + shortID() + shortID()
				_, _ = s.db.Exec(`UPDATE workspaces SET api_key = ? WHERE id = ?`, ws.APIKey, workspaceID)
			} else {
				ws.APIKey = recoveredAPIKey
			}
		} else {
			ws.Name = workspaceID
			ws.APIKey = "sk_" + shortID() + shortID()
			_, _ = s.db.Exec(`INSERT OR IGNORE INTO workspaces (id, name, api_key, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
				workspaceID, workspaceID, ws.APIKey, time.Now().UTC().Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339))
		}

		s.mu.Lock()
		s.workspaces[workspaceID] = ws
		s.mu.Unlock()

		if err := s.refreshWorkspaceStatus(ws); err != nil {
			log.Printf("workspace %s status refresh warning: %v", workspaceID, err)
		}
		go s.consumeAgentEvents(ws)
		log.Printf("Recovered workspace %s on port %d", workspaceID, agentPort)
	}
}
