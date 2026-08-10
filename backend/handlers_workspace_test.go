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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func setupTestServer(t *testing.T) (*Server, func()) {
	tmpDir, err := os.MkdirTemp("", "backend_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	db := InitDB(tmpDir)
	srv := NewServer(tmpDir, tmpDir, db)
	srv.routes()

	cleanup := func() {
		_ = db.Close()
		_ = os.RemoveAll(tmpDir)
	}
	return srv, cleanup
}

func TestHandleListWorkspaces(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	// Add test workspaces
	srv.mu.Lock()
	srv.workspaces["ws_1"] = &Workspace{
		ID:        "ws_1",
		Name:      "New Workspace",
		Status:    "active",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	srv.workspaces["ws_2"] = &Workspace{
		ID:        "ws_2",
		Name:      "New Workspace", // duplicate name allowed
		Status:    "active",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	srv.mu.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces", nil)
	rec := httptest.NewRecorder()

	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var res struct {
		Items []map[string]interface{} `json:"items"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(res.Items) != 2 {
		t.Fatalf("expected 2 workspaces, got %d", len(res.Items))
	}
}

func TestHandleRenameWorkspace(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	srv.mu.Lock()
	srv.workspaces["ws_1"] = &Workspace{
		ID:        "ws_1",
		Name:      "New Workspace",
		Status:    "active",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	srv.workspaces["ws_2"] = &Workspace{
		ID:        "ws_2",
		Name:      "Second Workspace",
		Status:    "active",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	srv.mu.Unlock()

	tests := []struct {
		name         string
		workspaceID  string
		payload      map[string]string
		wantCode     int
		wantName     string
		wantErrorKey string
	}{
		{
			name:        "rename ws_1 to custom name",
			workspaceID: "ws_1",
			payload:     map[string]string{"name": "Production Workspace"},
			wantCode:    http.StatusOK,
			wantName:    "Production Workspace",
		},
		{
			name:        "rename ws_1 to existing duplicate name (allowed)",
			workspaceID: "ws_1",
			payload:     map[string]string{"name": "Second Workspace"},
			wantCode:    http.StatusOK,
			wantName:    "Second Workspace",
		},
		{
			name:         "empty name validation error",
			workspaceID:  "ws_1",
			payload:      map[string]string{"name": "   "},
			wantCode:     http.StatusBadRequest,
			wantErrorKey: "WORKSPACE_NAME_REQUIRED",
		},
		{
			name:         "workspace not found",
			workspaceID:  "ws_nonexistent",
			payload:      map[string]string{"name": "Test"},
			wantCode:     http.StatusNotFound,
			wantErrorKey: "WORKSPACE_NOT_FOUND",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest(http.MethodPatch, "/api/v1/workspaces/"+tt.workspaceID, bytes.NewReader(body))
			rec := httptest.NewRecorder()

			srv.mux.ServeHTTP(rec, req)

			if rec.Code != tt.wantCode {
				t.Errorf("got status %d, want %d (body: %s)", rec.Code, tt.wantCode, rec.Body.String())
			}

			if tt.wantName != "" {
				var res struct {
					Ok   bool   `json:"ok"`
					Name string `json:"name"`
				}
				_ = json.NewDecoder(rec.Body).Decode(&res)
				if res.Name != tt.wantName {
					t.Errorf("got name %q, want %q", res.Name, tt.wantName)
				}
				// Verify in-memory state updated
				workspace, ok := srv.getWorkspace(tt.workspaceID)
				if !ok {
					t.Errorf("workspace not found in memory for id %q", tt.workspaceID)
				} else if workspace.Name != tt.wantName {
					t.Errorf("workspace in-memory name not updated: got %q, want %q", workspace.Name, tt.wantName)
				}
			}

			if tt.wantErrorKey != "" {
				var errRes struct {
					Type    string `json:"type"`
					Code    string `json:"code"`
					Message string `json:"message"`
				}
				_ = json.NewDecoder(rec.Body).Decode(&errRes)
				if errRes.Code != tt.wantErrorKey {
					t.Errorf("got error code %q, want %q", errRes.Code, tt.wantErrorKey)
				}
			}
		})
	}
}

func TestLoadAllWorkspaces(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "load_workspaces_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	db := InitDB(tmpDir)
	defer db.Close()

	// Mock HTTP server for health check
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
		}
	}))
	defer ts.Close()

	now := time.Now().UTC().Format(time.RFC3339)

	// Insert mock active workspace with valid health check
	_, err = db.Exec(`INSERT INTO workspaces (id, name, target_app, api_key, agent_port, container_id, agent_base_url, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"ws_persisted", "Persisted Workspace", "google_ads", "sk_test_persisted", 9999, "container_123", ts.URL, now, now)
	if err != nil {
		t.Fatalf("failed to insert mock workspace: %v", err)
	}

	// Insert mock dead workspace with invalid URL
	_, err = db.Exec(`INSERT INTO workspaces (id, name, target_app, api_key, agent_port, container_id, agent_base_url, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"ws_dead", "Dead Workspace", "google_ads", "sk_test_dead", 9998, "container_456", "http://127.0.0.1:54321", now, now)
	if err != nil {
		t.Fatalf("failed to insert dead workspace: %v", err)
	}

	srv := NewServer(tmpDir, tmpDir, db)
	srv.routes()

	srv.mu.RLock()
	ws, ok := srv.workspaces["ws_persisted"]
	srv.mu.RUnlock()

	if !ok {
		t.Fatalf("expected ws_persisted to be recovered on server startup")
	}
	if ws.Name != "Persisted Workspace" {
		t.Errorf("expected name 'Persisted Workspace', got '%s'", ws.Name)
	}
	if ws.APIKey != "sk_test_persisted" {
		t.Errorf("expected apiKey 'sk_test_persisted', got '%s'", ws.APIKey)
	}

	srv.mu.RLock()
	_, deadOk := srv.workspaces["ws_dead"]
	srv.mu.RUnlock()
	if deadOk {
		t.Fatalf("did not expect unreachable ws_dead to be added to active workspaces")
	}
}
