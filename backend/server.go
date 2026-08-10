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
	"database/sql"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Server struct {
	mux        *http.ServeMux
	httpClient *http.Client
	repoRoot   string
	dataDir    string
	db         *sql.DB
	config     *Config

	mu               sync.RWMutex
	workspaces       map[string]*Workspace
	actionSets       map[string]*ActionSet
	shadowopsEnabled bool

	imageBuildOnce sync.Once
	imageBuildErr  error

	cronScheduler *CronScheduler
}

func NewServer(repoRoot, dataDir string, db *sql.DB) *Server {
	cfg := LoadConfig(repoRoot)
	if dataDir != "" {
		cfg.DataDir = dataDir
	}
	return NewServerWithConfig(cfg, db)
}

func NewServerWithConfig(cfg *Config, db *sql.DB) *Server {
	repoRoot := cfg.RepoRoot
	if repoRoot == "" {
		repoRoot = filepath.Dir(cfg.DataDir)
	}
	s := &Server{
		mux:              http.NewServeMux(),
		httpClient:       &http.Client{Timeout: 5 * time.Second},
		repoRoot:         repoRoot,
		dataDir:          cfg.DataDir,
		db:               db,
		config:           cfg,
		workspaces:       map[string]*Workspace{},
		actionSets:       map[string]*ActionSet{},
		shadowopsEnabled: cfg.ShadowOpsEnabled,
	}
	s.loadAllWorkspaces()
	s.cronScheduler = NewCronScheduler(s)
	return s
}

func (s *Server) loadAllWorkspaces() {
	if s.db == nil {
		return
	}
	rows, err := s.db.Query("SELECT id, name, target_app, api_key, agent_port, container_id, agent_base_url, created_at, updated_at FROM workspaces")
	if err != nil {
		log.Printf("failed to load workspaces from db: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id string
		var name, targetApp, apiKey, containerID, agentBaseURL, createdAtStr, updatedAtStr sql.NullString
		var agentPort sql.NullInt64
		if err := rows.Scan(&id, &name, &targetApp, &apiKey, &agentPort, &containerID, &agentBaseURL, &createdAtStr, &updatedAtStr); err != nil {
			log.Printf("failed to scan workspace: %v", err)
			continue
		}

		wsName := name.String
		if wsName == "" {
			wsName = "Workspace"
		}
		wsTargetApp := targetApp.String
		if wsTargetApp == "" {
			wsTargetApp = "google_ads"
		}
		wsAPIKey := apiKey.String
		if wsAPIKey == "" {
			wsAPIKey = "sk_" + shortID()
		}

		createdAt, _ := time.Parse(time.RFC3339, createdAtStr.String)
		updatedAt, _ := time.Parse(time.RFC3339, updatedAtStr.String)
		if createdAt.IsZero() {
			createdAt = time.Now().UTC()
		}
		if updatedAt.IsZero() {
			updatedAt = time.Now().UTC()
		}

		port := int(agentPort.Int64)
		wsAgentBaseURL := agentBaseURL.String
		if wsAgentBaseURL == "" && port > 0 {
			wsAgentBaseURL = s.workspaceAgentURL(port)
		}
		if wsAgentBaseURL == "" {
			continue
		}

		ws := &Workspace{
			ID:                id,
			Name:              wsName,
			TargetApp:         wsTargetApp,
			APIKey:            wsAPIKey,
			Status:            "active",
			RecordingState:    "off",
			CreatedAt:         createdAt,
			UpdatedAt:         updatedAt,
			AgentPort:         port,
			AgentBaseURL:      wsAgentBaseURL,
			DockerContainerID: containerID.String,
			clients:           map[*WSClient]struct{}{},
		}

		// Check if agent is healthy
		if err := s.waitForAgentHealth(ws, 2*time.Second); err != nil {
			log.Printf("workspace %s agent not healthy at %s: %v", id, wsAgentBaseURL, err)
			continue
		}

		s.mu.Lock()
		s.workspaces[id] = ws
		s.mu.Unlock()

		_ = s.refreshWorkspaceStatus(ws)
		go s.consumeAgentEvents(ws)
		log.Printf("recovered workspace %s (name: %s, port: %d)", id, wsName, port)
	}
}

func (s *Server) serveStaticOrSPA(w http.ResponseWriter, r *http.Request) {
	if s.config == nil || s.config.StaticDir == "" {
		http.NotFound(w, r)
		return
	}

	cleanBase := filepath.Clean(s.config.StaticDir)
	cleanPath := filepath.Clean(strings.TrimPrefix(r.URL.Path, "/"))
	targetFile := filepath.Join(cleanBase, cleanPath)

	// Ensure targetFile stays within cleanBase to prevent directory traversal
	if !strings.HasPrefix(targetFile, cleanBase+string(filepath.Separator)) && targetFile != cleanBase {
		http.NotFound(w, r)
		return
	}

	if info, err := os.Stat(targetFile); err == nil && !info.IsDir() {
		http.ServeFile(w, r, targetFile)
		return
	}

	indexPath := filepath.Join(cleanBase, "index.html")
	if _, err := os.Stat(indexPath); err == nil {
		http.ServeFile(w, r, indexPath)
		return
	}

	http.NotFound(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":               true,
			"service":          "shadowops-backend",
			"shadowopsEnabled": s.shadowopsEnabled,
		})
	})
	s.mux.HandleFunc("GET /api/v1/workspaces", s.handleListWorkspaces)
	s.mux.HandleFunc("POST /api/v1/workspaces", s.handleCreateWorkspace)
	s.mux.HandleFunc("PATCH /api/v1/workspaces/{workspaceId}", s.handleRenameWorkspace)
	s.mux.HandleFunc("GET /api/v1/workspaces/{workspaceId}", s.handleGetWorkspace)
	s.mux.HandleFunc("POST /api/v1/workspaces/{workspaceId}/navigate", s.handleNavigateWorkspace)
	s.mux.HandleFunc("POST /api/v1/workspaces/{workspaceId}/kill", s.handleKillWorkspace)
	s.mux.HandleFunc("POST /api/v1/workspaces/{workspaceId}/shadow/start", s.handleShadowStart)
	s.mux.HandleFunc("POST /api/v1/workspaces/{workspaceId}/shadow/end", s.handleShadowEnd)
	s.mux.HandleFunc("GET /api/v1/workspaces/{workspaceId}/shadow/actions", s.handleShadowActions)
	s.mux.HandleFunc("POST /api/v1/workspaces/{workspaceId}/script/run", s.handleScriptRun)
	s.mux.HandleFunc("GET /api/v1/workspaces/{workspaceId}/script/status", s.handleScriptStatus)
	s.mux.HandleFunc("POST /api/v1/workspaces/{workspaceId}/script/cancel", s.handleScriptCancel)
	s.mux.HandleFunc("GET /api/v1/workspaces/{workspaceId}/ws", s.handleWorkspaceWS)
	s.mux.HandleFunc("GET /api/v1/default-scripts", s.handleListDefaultScripts)

	s.mux.HandleFunc("GET /api/v1/workspaces/{workspaceId}/cron", s.handleGetWorkspaceCron)
	s.mux.HandleFunc("POST /api/v1/workspaces/{workspaceId}/cron", s.handleSetWorkspaceCron)
	s.mux.HandleFunc("POST /api/v1/workspaces/{workspaceId}/upload-csv", s.handleUploadWorkspaceCSV)
	s.mux.HandleFunc("DELETE /api/v1/workspaces/{workspaceId}/cron", s.handleDeleteWorkspaceCron)
	s.mux.HandleFunc("GET /api/v1/workspaces/{workspaceId}/executions", s.handleListBatchExecutions)
	s.mux.HandleFunc("GET /api/v1/workspaces/{workspaceId}/executions/{id}/csv", s.handleDownloadBatchExecutionCsv)
	s.mux.HandleFunc("POST /api/v1/workspaces/{workspaceId}/executions/{id}/cancel", s.handleCancelBatchExecution)

	s.mux.HandleFunc("GET /api/v1/action-sets", s.handleListActionSets)
	s.mux.HandleFunc("POST /api/v1/action-sets", s.handleCreateActionSet)
	s.mux.HandleFunc("GET /api/v1/action-sets/{actionSetId}", s.handleGetActionSet)
	s.mux.HandleFunc("PUT /api/v1/action-sets/{actionSetId}", s.handleUpdateActionSet)
	s.mux.HandleFunc("DELETE /api/v1/action-sets/{actionSetId}", s.handleDeleteActionSet)
	s.mux.HandleFunc("POST /api/v1/action-sets/{actionSetId}/generate-script", s.handleGenerateScript)

	// If a static directory exists for the Angular frontend UI, serve it with SPA fallback
	if s.config != nil && s.config.StaticDir != "" {
		if info, err := os.Stat(s.config.StaticDir); err == nil && info.IsDir() {
			s.mux.HandleFunc("GET /", s.serveStaticOrSPA)
		}
	}
}
