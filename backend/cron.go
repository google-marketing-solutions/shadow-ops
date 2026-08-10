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
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

type WorkspaceCron struct {
	WorkspaceID    string `json:"workspaceId"`
	CronExpression string `json:"cronExpression"`
	ScriptFilename string `json:"scriptFilename"`
	ScriptSource   string `json:"scriptSource"`
	CsvFilename    string `json:"csvFilename"`
	CsvData        string `json:"csvData"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
}

type BatchExecution struct {
	ID            string `json:"id"`
	WorkspaceID   string `json:"workspaceId"`
	StartedAt     string `json:"startedAt"`
	FinishedAt    string `json:"finishedAt"`
	Status        string `json:"status"`
	ResultCsvData string `json:"resultCsvData"`
}

type CronScheduler struct {
	server      *Server
	cron        *cron.Cron
	jobs        map[string]cron.EntryID
	activeExecs map[string]context.CancelFunc
	mu          sync.Mutex
}

func NewCronScheduler(server *Server) *CronScheduler {
	return &CronScheduler{
		server:      server,
		cron:        cron.New(),
		jobs:        make(map[string]cron.EntryID),
		activeExecs: make(map[string]context.CancelFunc),
	}
}

func (cs *CronScheduler) Start() {
	cs.cron.Start()
	cs.loadAllCrons()
}

func (cs *CronScheduler) loadAllCrons() {
	rows, err := cs.server.db.Query("SELECT workspace_id, cron_expression, script_filename, script_source, csv_filename, csv_data FROM workspace_crons")
	if err != nil {
		log.Printf("failed to load crons: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var wc WorkspaceCron
		if err := rows.Scan(&wc.WorkspaceID, &wc.CronExpression, &wc.ScriptFilename, &wc.ScriptSource, &wc.CsvFilename, &wc.CsvData); err != nil {
			log.Printf("failed to scan cron: %v", err)
			continue
		}
		cs.Schedule(wc)
	}
}

func (cs *CronScheduler) Schedule(wc WorkspaceCron) error {
	if id, ok := cs.jobs[wc.WorkspaceID]; ok {
		cs.cron.Remove(id)
	}

	id, err := cs.cron.AddFunc(wc.CronExpression, func() {
		cs.executeBatch(wc)
	})
	if err != nil {
		return err
	}
	cs.jobs[wc.WorkspaceID] = id
	return nil
}

func (cs *CronScheduler) Remove(workspaceID string) {
	if id, ok := cs.jobs[workspaceID]; ok {
		cs.cron.Remove(id)
		delete(cs.jobs, workspaceID)
	}
}

func (cs *CronScheduler) CancelExecution(workspaceID, execID string) {
	cs.mu.Lock()
	cancel, ok := cs.activeExecs[execID]
	cs.mu.Unlock()

	if ok {
		cancel()
	}

	// Also try to cancel the script on the workspace agent
	if ws, ok := cs.server.getWorkspace(workspaceID); ok {
		var resp map[string]any
		cs.server.agentPostJSON(ws, "/script/cancel", map[string]any{}, &resp)
	}
}

func (cs *CronScheduler) executeBatch(wc WorkspaceCron) {
	log.Printf("Starting batch execution for workspace %s", wc.WorkspaceID)

	// Check if there is already a running execution
	var count int
	err := cs.server.db.QueryRow("SELECT COUNT(*) FROM batch_executions WHERE workspace_id = ? AND status = 'running'", wc.WorkspaceID).Scan(&count)
	if err == nil && count > 0 {
		log.Printf("Skipping cron execution for workspace %s: another batch is already running", wc.WorkspaceID)
		return
	}

	execID := fmt.Sprintf("exec_%s", shortID())
	startedAt := time.Now().UTC().Format(time.RFC3339)

	ctx, cancel := context.WithCancel(context.Background())
	cs.mu.Lock()
	cs.activeExecs[execID] = cancel
	cs.mu.Unlock()

	defer func() {
		cs.mu.Lock()
		delete(cs.activeExecs, execID)
		cs.mu.Unlock()
		cancel()
	}()

	_, err = cs.server.db.Exec(`
		INSERT INTO batch_executions (id, workspace_id, started_at, status)
		VALUES (?, ?, ?, ?)`,
		execID, wc.WorkspaceID, startedAt, "running",
	)
	if err != nil {
		log.Printf("failed to insert batch execution: %v", err)
		return
	}

	ws, ok := cs.server.getWorkspace(wc.WorkspaceID)
	if !ok {
		cs.finishExecution(execID, "failed", "workspace not running")
		return
	}

	reader := csv.NewReader(strings.NewReader(wc.CsvData))
	records, err := reader.ReadAll()
	if err != nil || len(records) < 2 {
		cs.finishExecution(execID, "failed", "invalid csv data")
		return
	}

	headers := records[0]

	// We will append new columns dynamically
	newHeaders := append([]string{}, headers...)
	var outputRecords [][]string
	outputRecords = append(outputRecords, newHeaders)

	for i := 1; i < len(records); i++ {
		select {
		case <-ctx.Done():
			log.Printf("Batch execution %s was cancelled", execID)
			cs.finishExecution(execID, "cancelled", "")
			return
		default:
		}

		row := records[i]
		inputs := make(map[string]any)
		for idx, h := range headers {
			if idx < len(row) {
				inputs[h] = row[idx]
			} else {
				inputs[h] = ""
			}
		}

		// Run script
		var resp map[string]any
		scriptName := wc.ScriptFilename
		if scriptName == "" {
			scriptName = "cron_automation.py"
		}
		err := cs.server.agentPostJSON(ws, "/script/run", map[string]any{
			"filename": scriptName,
			"script":   wc.ScriptSource,
			"inputs":   inputs,
		}, &resp)

		if err != nil {
			row = append(row, fmt.Sprintf("Error: %v", err))
			outputRecords = append(outputRecords, row)
			continue
		}

		// Poll for status
		var finalStatus map[string]any
		for {
			select {
			case <-ctx.Done():
				log.Printf("Batch execution %s was cancelled during polling", execID)
				cs.finishExecution(execID, "cancelled", "")
				return
			case <-time.After(2 * time.Second):
			}

			var statusResp map[string]any
			if err := cs.server.agentGetJSON(ws, "/script/status", &statusResp); err != nil {
				continue
			}
			statusStr, _ := statusResp["status"].(string)
			if statusStr == "succeeded" || statusStr == "failed" || statusStr == "cancelled" {
				finalStatus = statusResp
				break
			}
		}

		statusStr, _ := finalStatus["status"].(string)
		if statusStr == "succeeded" {
			resultObj, ok := finalStatus["result"].(map[string]any)
			if ok {
				flatResult := flattenObject(resultObj, "")
				// Merge into row
				for k, v := range flatResult {
					// Find or add header
					colIdx := -1
					for j, h := range outputRecords[0] {
						if h == k {
							colIdx = j
							break
						}
					}
					if colIdx == -1 {
						outputRecords[0] = append(outputRecords[0], k)
						colIdx = len(outputRecords[0]) - 1
					}

					// Ensure row is long enough
					for len(row) <= colIdx {
						row = append(row, "")
					}
					row[colIdx] = fmt.Sprintf("%v", v)
				}
			} else {
				// Result is not an object, just append it
				colIdx := -1
				for j, h := range outputRecords[0] {
					if h == "Result" {
						colIdx = j
						break
					}
				}
				if colIdx == -1 {
					outputRecords[0] = append(outputRecords[0], "Result")
					colIdx = len(outputRecords[0]) - 1
				}
				for len(row) <= colIdx {
					row = append(row, "")
				}
				row[colIdx] = fmt.Sprintf("%v", finalStatus["result"])
			}
		} else {
			errorMsg, _ := finalStatus["error"].(string)
			if errorMsg == "" {
				errorMsg = "Script failed"
			}
			colIdx := -1
			for j, h := range outputRecords[0] {
				if h == "Error" {
					colIdx = j
					break
				}
			}
			if colIdx == -1 {
				outputRecords[0] = append(outputRecords[0], "Error")
				colIdx = len(outputRecords[0]) - 1
			}
			for len(row) <= colIdx {
				row = append(row, "")
			}
			row[colIdx] = errorMsg
		}

		// Pad row to match header length
		for len(row) < len(outputRecords[0]) {
			row = append(row, "")
		}
		outputRecords = append(outputRecords, row)
	}

	// Pad all previous rows to match final header length
	for i := 1; i < len(outputRecords); i++ {
		for len(outputRecords[i]) < len(outputRecords[0]) {
			outputRecords[i] = append(outputRecords[i], "")
		}
	}

	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	writer.WriteAll(outputRecords)
	writer.Flush()

	cs.finishExecution(execID, "success", buf.String())
}

func (cs *CronScheduler) finishExecution(execID, status, resultData string) {
	finishedAt := time.Now().UTC().Format(time.RFC3339)
	_, err := cs.server.db.Exec(`
		UPDATE batch_executions
		SET finished_at = ?, status = ?, result_csv_data = ?
		WHERE id = ?`,
		finishedAt, status, resultData, execID,
	)
	if err != nil {
		log.Printf("failed to update batch execution: %v", err)
	}

	// Keep only last 10 executions
	var workspaceID string
	err = cs.server.db.QueryRow("SELECT workspace_id FROM batch_executions WHERE id = ?", execID).Scan(&workspaceID)
	if err == nil {
		cs.server.db.Exec(`
			DELETE FROM batch_executions
			WHERE workspace_id = ? AND id NOT IN (
				SELECT id FROM batch_executions
				WHERE workspace_id = ?
				ORDER BY started_at DESC
				LIMIT 10
			)`, workspaceID, workspaceID)
	}
}

func flattenObject(obj map[string]any, prefix string) map[string]any {
	result := make(map[string]any)
	for k, v := range obj {
		pre := ""
		if prefix != "" {
			pre = prefix + "_"
		}
		if nestedMap, ok := v.(map[string]any); ok {
			flatNested := flattenObject(nestedMap, pre+k)
			for nk, nv := range flatNested {
				result[nk] = nv
			}
		} else {
			result[pre+k] = v
		}
	}
	return result
}
