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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestProcessAgentEvent(t *testing.T) {
	s := &Server{}
	ws := &Workspace{
		ID:             "ws_test_events",
		Status:         "active",
		RecordingState: "off",
		CurrentURL:     "https://example.com",
		clients:        map[*WSClient]struct{}{},
	}

	tests := []struct {
		name          string
		payload       map[string]any
		wantStatus    string
		wantRecording string
		wantURL       string
		wantSteps     int
	}{
		{
			name: "workspace.status event updates workspace fields",
			payload: map[string]any{
				"type":           "workspace.status",
				"status":         "recording",
				"recordingState": "on",
				"currentUrl":     "https://ads.google.com",
			},
			wantStatus:    "recording",
			wantRecording: "on",
			wantURL:       "https://ads.google.com",
			wantSteps:     0,
		},
		{
			name: "shadow.step event appends step to workspace",
			payload: map[string]any{
				"type": "shadow.step",
				"step": map[string]any{
					"stepIndex": 1,
					"type":      "click",
					"target":    "button#submit",
				},
			},
			wantStatus:    "recording",
			wantRecording: "on",
			wantURL:       "https://ads.google.com",
			wantSteps:     1,
		},
		{
			name: "invalid event type does not corrupt workspace",
			payload: map[string]any{
				"type": "unknown.event",
			},
			wantStatus:    "recording",
			wantRecording: "on",
			wantURL:       "https://ads.google.com",
			wantSteps:     1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data, err := json.Marshal(tc.payload)
			if err != nil {
				t.Fatalf("failed to marshal payload: %v", err)
			}
			s.processAgentEvent(ws, data)

			ws.mu.RLock()
			defer ws.mu.RUnlock()
			if ws.Status != tc.wantStatus {
				t.Errorf("Status = %q, want %q", ws.Status, tc.wantStatus)
			}
			if ws.RecordingState != tc.wantRecording {
				t.Errorf("RecordingState = %q, want %q", ws.RecordingState, tc.wantRecording)
			}
			if ws.CurrentURL != tc.wantURL {
				t.Errorf("CurrentURL = %q, want %q", ws.CurrentURL, tc.wantURL)
			}
			if len(ws.steps) != tc.wantSteps {
				t.Errorf("len(steps) = %d, want %d", len(ws.steps), tc.wantSteps)
			}
		})
	}
}

func TestBroadcastRawAndEvent(t *testing.T) {
	s := &Server{}
	ws := &Workspace{
		ID:      "ws_test_broadcast",
		clients: map[*WSClient]struct{}{},
	}

	up := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	var serverConn *websocket.Conn
	var clientConn *websocket.Conn

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		serverConn, err = up.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade error: %v", err)
		}
	}))
	defer ts.Close()

	u, _ := url.Parse(ts.URL)
	u.Scheme = "ws"

	var err error
	clientConn, _, err = websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		t.Fatalf("dial error: %v", err)
	}
	defer clientConn.Close()

	wsClient := &WSClient{conn: serverConn}
	ws.mu.Lock()
	ws.clients[wsClient] = struct{}{}
	ws.mu.Unlock()

	defer func() {
		if serverConn != nil {
			_ = serverConn.Close()
		}
	}()

	t.Run("broadcastEvent sends json payload to connected client", func(t *testing.T) {
		msg := map[string]any{"type": "test", "value": 123}
		s.broadcastEvent(ws, msg)

		_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, received, err := clientConn.ReadMessage()
		if err != nil {
			t.Fatalf("failed to read broadcast message: %v", err)
		}
		if !strings.Contains(string(received), `"value":123`) {
			t.Errorf("expected payload with value:123, got %s", string(received))
		}
	})
}
