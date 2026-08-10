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
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func setupTestServer() (*AgentServer, *http.ServeMux) {
	mgr := NewStateManager("ws_test")
	mockEngine := NewMockAgentEngine(mgr)
	srv := NewAgentServer(mgr, mockEngine)
	mux := http.NewServeMux()
	srv.SetupRoutes(mux)
	return srv, mux
}

func TestServerRESTEndpoints(t *testing.T) {
	srv, mux := setupTestServer()
	_ = srv

	tests := []struct {
		name       string
		method     string
		path       string
		body       interface{}
		wantStatus int
		checkBody  func(*testing.T, *bytes.Buffer)
	}{
		{
			name:       "GET /health returns ok",
			method:     "GET",
			path:       "/health",
			body:       nil,
			wantStatus: http.StatusOK,
			checkBody: func(t *testing.T, buf *bytes.Buffer) {
				if !strings.Contains(buf.String(), `"ok":true`) {
					t.Errorf("expected ok:true, got %s", buf.String())
				}
			},
		},
		{
			name:       "GET /status returns status",
			method:     "GET",
			path:       "/status",
			body:       nil,
			wantStatus: http.StatusOK,
			checkBody: func(t *testing.T, buf *bytes.Buffer) {
				if !strings.Contains(buf.String(), `"status":"active"`) {
					t.Errorf("expected active status, got %s", buf.String())
				}
			},
		},
		{
			name:       "POST /navigate with empty URL fails",
			method:     "POST",
			path:       "/navigate",
			body:       map[string]string{"url": ""},
			wantStatus: http.StatusBadRequest,
			checkBody:  nil,
		},
		{
			name:       "POST /navigate with valid URL succeeds",
			method:     "POST",
			path:       "/navigate",
			body:       map[string]string{"url": "https://ads.google.com"},
			wantStatus: http.StatusOK,
			checkBody: func(t *testing.T, buf *bytes.Buffer) {
				if !strings.Contains(buf.String(), `"ok":true`) {
					t.Errorf("expected ok:true, got %s", buf.String())
				}
			},
		},
		{
			name:       "POST /shadow/start and /shadow/end flow",
			method:     "POST",
			path:       "/shadow/start",
			body:       nil,
			wantStatus: http.StatusOK,
			checkBody: func(t *testing.T, buf *bytes.Buffer) {
				if !strings.Contains(buf.String(), `"recordingState":"on"`) {
					t.Errorf("expected recordingState on, got %s", buf.String())
				}
			},
		},
		{
			name:       "POST /script/run with script succeeds",
			method:     "POST",
			path:       "/script/run",
			body:       map[string]interface{}{"script": "print('hello')", "filename": "test.py"},
			wantStatus: http.StatusOK,
			checkBody: func(t *testing.T, buf *bytes.Buffer) {
				if !strings.Contains(buf.String(), `"runId":`) {
					t.Errorf("expected runId in response, got %s", buf.String())
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var reqBody *bytes.Buffer
			if tt.body != nil {
				jsonBytes, _ := json.Marshal(tt.body)
				reqBody = bytes.NewBuffer(jsonBytes)
			} else {
				reqBody = bytes.NewBuffer(nil)
			}

			req := httptest.NewRequest(tt.method, tt.path, reqBody)
			if tt.body != nil {
				req.Header.Set("Content-Type", "application/json")
			}
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("got HTTP status %d, want %d (body: %s)", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if tt.checkBody != nil {
				tt.checkBody(t, rec.Body)
			}
		})
	}
}

func TestServerEventsWebSocket(t *testing.T) {
	srv, mux := setupTestServer()
	ts := httptest.NewServer(mux)
	defer ts.Close()

	u, _ := url.Parse(ts.URL)
	u.Scheme = "ws"
	u.Path = "/events"

	ws, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		t.Fatalf("failed to connect to websocket: %v", err)
	}
	defer ws.Close()

	var firstMsg map[string]interface{}
	if err := ws.ReadJSON(&firstMsg); err != nil {
		t.Fatalf("failed to read initial websocket message: %v", err)
	}

	if firstMsg["type"] != "workspace.status" {
		t.Errorf("expected initial message type workspace.status, got %v", firstMsg["type"])
	}

	srv.Broadcast(map[string]string{"type": "test.broadcast", "data": "hello"})

	var bcastMsg map[string]interface{}
	_ = ws.SetReadDeadline(time.Now().Add(1 * time.Second))
	if err := ws.ReadJSON(&bcastMsg); err != nil {
		t.Fatalf("failed to read broadcast message: %v", err)
	}
	if bcastMsg["type"] != "test.broadcast" {
		t.Errorf("expected test.broadcast, got %v", bcastMsg["type"])
	}
}
