package main

import (
	"database/sql"
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
	s.cronScheduler = NewCronScheduler(s)
	return s
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
