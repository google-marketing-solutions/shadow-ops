package main

import (
	"database/sql"
	"net/http"
	"os"
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

	mu               sync.RWMutex
	workspaces       map[string]*Workspace
	actionSets       map[string]*ActionSet
	shadowopsEnabled bool

	imageBuildOnce sync.Once
	imageBuildErr  error

	cronScheduler *CronScheduler
}

func NewServer(repoRoot, dataDir string, db *sql.DB) *Server {
	s := &Server{
		mux:              http.NewServeMux(),
		httpClient:       &http.Client{Timeout: 5 * time.Second},
		repoRoot:         repoRoot,
		dataDir:          dataDir,
		db:               db,
		workspaces:       map[string]*Workspace{},
		actionSets:       map[string]*ActionSet{},
		shadowopsEnabled: strings.EqualFold(strings.TrimSpace(os.Getenv("SHADOWOPS_ENABLED")), "true"),
	}
	s.cronScheduler = NewCronScheduler(s)
	return s
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
}
