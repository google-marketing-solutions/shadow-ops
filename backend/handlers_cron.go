package main

import (
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

func (s *Server) handleGetWorkspaceCron(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceId")

	var wc WorkspaceCron
	err := s.db.QueryRow(`
		SELECT workspace_id, cron_expression, script_filename, script_source, csv_filename, csv_data, created_at, updated_at
		FROM workspace_crons
		WHERE workspace_id = ?`, workspaceID).Scan(
		&wc.WorkspaceID, &wc.CronExpression, &wc.ScriptFilename, &wc.ScriptSource, &wc.CsvFilename, &wc.CsvData, &wc.CreatedAt, &wc.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "CRON_NOT_FOUND", "No cron schedule found for this workspace")
			return
		}
		writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, wc)
}

func (s *Server) handleSetWorkspaceCron(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceId")

	var req struct {
		CronExpression string `json:"cronExpression"`
		ScriptFilename string `json:"scriptFilename"`
		ScriptSource   string `json:"scriptSource"`
		CsvFilename    string `json:"csvFilename"`
		CsvData        string `json:"csvData"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON")
		return
	}

	var existing struct {
		ScriptFilename string
		ScriptSource   string
		CsvFilename    string
		CsvData        string
	}
	err := s.db.QueryRow("SELECT script_filename, script_source, csv_filename, csv_data FROM workspace_crons WHERE workspace_id = ?", workspaceID).
		Scan(&existing.ScriptFilename, &existing.ScriptSource, &existing.CsvFilename, &existing.CsvData)
	if err == nil {
		if req.ScriptFilename == "" { req.ScriptFilename = existing.ScriptFilename }
		if req.ScriptSource == "" { req.ScriptSource = existing.ScriptSource }
		if req.CsvFilename == "" { req.CsvFilename = existing.CsvFilename }
		if req.CsvData == "" { req.CsvData = existing.CsvData }
	}

	now := time.Now().UTC().Format(time.RFC3339)
	wc := WorkspaceCron{
		WorkspaceID:    workspaceID,
		CronExpression: req.CronExpression,
		ScriptFilename: req.ScriptFilename,
		ScriptSource:   req.ScriptSource,
		CsvFilename:    req.CsvFilename,
		CsvData:        req.CsvData,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	_, err = s.db.Exec(`
		INSERT INTO workspace_crons (workspace_id, cron_expression, script_filename, script_source, csv_filename, csv_data, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(workspace_id) DO UPDATE SET
			cron_expression = excluded.cron_expression,
			script_filename = excluded.script_filename,
			script_source = excluded.script_source,
			csv_filename = excluded.csv_filename,
			csv_data = excluded.csv_data,
			updated_at = excluded.updated_at`,
		wc.WorkspaceID, wc.CronExpression, wc.ScriptFilename, wc.ScriptSource, wc.CsvFilename, wc.CsvData, wc.CreatedAt, wc.UpdatedAt,
	)

	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	if err := s.cronScheduler.Schedule(wc); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_CRON", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleDeleteWorkspaceCron(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceId")

	_, err := s.db.Exec("DELETE FROM workspace_crons WHERE workspace_id = ?", workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	s.cronScheduler.Remove(workspaceID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleListBatchExecutions(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceId")

	rows, err := s.db.Query(`
		SELECT id, workspace_id, started_at, finished_at, status
		FROM batch_executions
		WHERE workspace_id = ?
		ORDER BY started_at DESC
		LIMIT 10`, workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	defer rows.Close()

	var executions []map[string]any
	for rows.Next() {
		var id, wsID, startedAt, status string
		var finishedAt sql.NullString
		if err := rows.Scan(&id, &wsID, &startedAt, &finishedAt, &status); err != nil {
			continue
		}
		executions = append(executions, map[string]any{
			"id":          id,
			"workspaceId": wsID,
			"startedAt":   startedAt,
			"finishedAt":  finishedAt.String,
			"status":      status,
		})
	}

	if executions == nil {
		executions = []map[string]any{}
	}

	writeJSON(w, http.StatusOK, executions)
}

func (s *Server) handleDownloadBatchExecutionCsv(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var csvData sql.NullString
	err := s.db.QueryRow("SELECT result_csv_data FROM batch_executions WHERE id = ?", id).Scan(&csvData)
	if err != nil {
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "EXECUTION_NOT_FOUND", "Execution not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", `attachment; filename="batch_result.csv"`)
	w.Write([]byte(csvData.String))
}

func (s *Server) handleCancelBatchExecution(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceId")
	id := r.PathValue("id")

	s.cronScheduler.CancelExecution(workspaceID, id)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleUploadWorkspaceCSV(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceId")

	s.mu.RLock()
	ws, ok := s.workspaces[workspaceID]
	s.mu.RUnlock()
	if !ok {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Workspace not found")
		return
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid Authorization header")
		return
	}
	token := strings.TrimPrefix(authHeader, "Bearer ")

	ws.mu.RLock()
	apiKey := ws.APIKey
	ws.mu.RUnlock()

	if apiKey == "" || subtle.ConstantTimeCompare([]byte(token), []byte(apiKey)) != 1 {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid API Key")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "File size exceeds 10MB limit or invalid multipart form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "Missing 'file' field")
		return
	}
	defer file.Close()

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "READ_ERROR", "Failed to read file")
		return
	}

	csvFilename := header.Filename
	csvData := string(fileBytes)
	now := time.Now().UTC().Format(time.RFC3339)

	_, err = s.db.Exec(`
		INSERT INTO workspace_crons (workspace_id, cron_expression, script_filename, script_source, csv_filename, csv_data, created_at, updated_at)
		VALUES (?, '', '', '', ?, ?, ?, ?)
		ON CONFLICT(workspace_id) DO UPDATE SET
			csv_filename = excluded.csv_filename,
			csv_data = excluded.csv_data,
			updated_at = excluded.updated_at`,
		workspaceID, csvFilename, csvData, now, now,
	)

	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	var wc WorkspaceCron
	err = s.db.QueryRow("SELECT cron_expression, script_filename, script_source FROM workspace_crons WHERE workspace_id = ?", workspaceID).
		Scan(&wc.CronExpression, &wc.ScriptFilename, &wc.ScriptSource)
	if err == nil && strings.TrimSpace(wc.CronExpression) != "" {
		wc.WorkspaceID = workspaceID
		wc.CsvFilename = csvFilename
		wc.CsvData = csvData
		_ = s.cronScheduler.Schedule(wc)
	}

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}
