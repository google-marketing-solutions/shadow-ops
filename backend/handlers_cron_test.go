package main

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func setupCronTestServer(t *testing.T) (*Server, func()) {
	tmpDir, err := os.MkdirTemp("", "backend_cron_test_*")
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

func TestHandleUploadWorkspaceCSV(t *testing.T) {
	srv, cleanup := setupCronTestServer(t)
	defer cleanup()

	srv.mu.Lock()
	srv.workspaces["ws_test"] = &Workspace{
		ID:        "ws_test",
		Name:      "Test Workspace",
		APIKey:    "sk_testkey123456",
		Status:    "active",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	srv.mu.Unlock()

	tests := []struct {
		name        string
		workspaceID string
		authHeader  string
		csvContent  string
		hasFile     bool
		wantCode    int
	}{
		{
			name:        "valid csv upload with bearer token",
			workspaceID: "ws_test",
			authHeader:  "Bearer sk_testkey123456",
			csvContent:  "col1,col2\nval1,val2\n",
			hasFile:     true,
			wantCode:    http.StatusOK,
		},
		{
			name:        "missing auth header",
			workspaceID: "ws_test",
			authHeader:  "",
			csvContent:  "col1,col2\n",
			hasFile:     true,
			wantCode:    http.StatusUnauthorized,
		},
		{
			name:        "invalid api key",
			workspaceID: "ws_test",
			authHeader:  "Bearer sk_wrongkey",
			csvContent:  "col1,col2\n",
			hasFile:     true,
			wantCode:    http.StatusUnauthorized,
		},
		{
			name:        "nonexistent workspace",
			workspaceID: "ws_nonexistent",
			authHeader:  "Bearer sk_testkey123456",
			csvContent:  "col1,col2\n",
			hasFile:     true,
			wantCode:    http.StatusNotFound,
		},
		{
			name:        "missing file field",
			workspaceID: "ws_test",
			authHeader:  "Bearer sk_testkey123456",
			csvContent:  "",
			hasFile:     false,
			wantCode:    http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)

			if tt.hasFile {
				part, err := writer.CreateFormFile("file", "test.csv")
				if err != nil {
					t.Fatalf("failed to create form file: %v", err)
				}
				if _, err := part.Write([]byte(tt.csvContent)); err != nil {
					t.Fatalf("failed to write part content: %v", err)
				}
			}
			_ = writer.Close()

			req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/"+tt.workspaceID+"/upload-csv", body)
			req.Header.Set("Content-Type", writer.FormDataContentType())
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			rec := httptest.NewRecorder()
			srv.mux.ServeHTTP(rec, req)

			if rec.Code != tt.wantCode {
				t.Errorf("got status %d, want %d (body: %s)", rec.Code, tt.wantCode, rec.Body.String())
			}

			if tt.wantCode == http.StatusOK {
				var res map[string]bool
				if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
					t.Fatalf("failed to decode response JSON: %v", err)
				}
				if !res["success"] {
					t.Errorf("expected success to be true, got %v", res["success"])
				}

				// Verify in database
				var dbCsvData string
				err := srv.db.QueryRow("SELECT csv_data FROM workspace_crons WHERE workspace_id = ?", tt.workspaceID).Scan(&dbCsvData)
				if err != nil {
					t.Errorf("failed to query database for uploaded csv: %v", err)
				} else if dbCsvData != tt.csvContent {
					t.Errorf("got csv_data %q, want %q", dbCsvData, tt.csvContent)
				}
			}
		})
	}
}
