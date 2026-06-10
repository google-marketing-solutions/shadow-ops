package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"
)

func (s *Server) handleGetWorkspaceCron(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceId")

	var wc WorkspaceCron
	err := s.db.QueryRow(`
		SELECT workspace_id, cron_expression, script_filename, script_source, csv_filename, csv_data, url_column, created_at, updated_at
		FROM workspace_crons
		WHERE workspace_id = ?`, workspaceID).Scan(
		&wc.WorkspaceID, &wc.CronExpression, &wc.ScriptFilename, &wc.ScriptSource, &wc.CsvFilename, &wc.CsvData, &wc.UrlColumn, &wc.CreatedAt, &wc.UpdatedAt,
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
		UrlColumn      string `json:"urlColumn"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON")
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)
	wc := WorkspaceCron{
		WorkspaceID:    workspaceID,
		CronExpression: req.CronExpression,
		ScriptFilename: req.ScriptFilename,
		ScriptSource:   req.ScriptSource,
		CsvFilename:    req.CsvFilename,
		CsvData:        req.CsvData,
		UrlColumn:      req.UrlColumn,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	_, err := s.db.Exec(`
		INSERT INTO workspace_crons (workspace_id, cron_expression, script_filename, script_source, csv_filename, csv_data, url_column, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(workspace_id) DO UPDATE SET
			cron_expression = excluded.cron_expression,
			script_filename = excluded.script_filename,
			script_source = excluded.script_source,
			csv_filename = excluded.csv_filename,
			csv_data = excluded.csv_data,
			url_column = excluded.url_column,
			updated_at = excluded.updated_at`,
		wc.WorkspaceID, wc.CronExpression, wc.ScriptFilename, wc.ScriptSource, wc.CsvFilename, wc.CsvData, wc.UrlColumn, wc.CreatedAt, wc.UpdatedAt,
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

	var csvData string
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
	w.Write([]byte(csvData))
}

func (s *Server) handleCancelBatchExecution(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceId")
	id := r.PathValue("id")

	s.cronScheduler.CancelExecution(workspaceID, id)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
