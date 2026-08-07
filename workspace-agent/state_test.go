package main

import (
	"testing"
)

func TestNewInitialAgentState(t *testing.T) {
	tests := []struct {
		name        string
		workspaceID string
		wantID      string
		wantStatus  string
	}{
		{
			name:        "custom workspace id",
			workspaceID: "ws_test_123",
			wantID:      "ws_test_123",
			wantStatus:  "active",
		},
		{
			name:        "empty workspace id default fallback",
			workspaceID: "",
			wantID:      "ws_local",
			wantStatus:  "active",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := NewInitialAgentState(tt.workspaceID)
			if st.WorkspaceID != tt.wantID {
				t.Errorf("got WorkspaceID %q, want %q", st.WorkspaceID, tt.wantID)
			}
			if st.Status != tt.wantStatus {
				t.Errorf("got Status %q, want %q", st.Status, tt.wantStatus)
			}
			if st.RecordingState != "off" {
				t.Errorf("got RecordingState %q, want 'off'", st.RecordingState)
			}
			if st.ViewportWidth <= 0 || st.ViewportHeight <= 0 {
				t.Errorf("invalid viewport dimensions: %dx%d", st.ViewportWidth, st.ViewportHeight)
			}
		})
	}
}

func TestStateManager(t *testing.T) {
	mgr := NewStateManager("ws_mgr_test")

	t.Run("GetState returns valid clone", func(t *testing.T) {
		st := mgr.GetState()
		if st.WorkspaceID != "ws_mgr_test" {
			t.Errorf("unexpected WorkspaceID: %s", st.WorkspaceID)
		}
	})

	t.Run("UpdateState mutates state safely", func(t *testing.T) {
		updated := mgr.UpdateState(func(s *AgentState) {
			s.Status = "recording"
			s.RecordingState = "on"
			s.CurrentURL = "https://ads.google.com"
		})

		if updated.Status != "recording" || updated.RecordingState != "on" {
			t.Errorf("unexpected updated status: %+v", updated)
		}
		if updated.CurrentURL != "https://ads.google.com" {
			t.Errorf("unexpected url: %s", updated.CurrentURL)
		}
	})

	t.Run("AddStep adds and indexes shadow step", func(t *testing.T) {
		step1 := mgr.AddStep(ShadowStep{
			Type:    "navigate",
			Payload: map[string]interface{}{"url": "https://ads.google.com"},
		})
		if step1.StepIndex != 1 {
			t.Errorf("expected stepIndex 1, got %d", step1.StepIndex)
		}
		if step1.Meta["url"] != "https://ads.google.com" {
			t.Errorf("expected step meta url https://ads.google.com, got %v", step1.Meta["url"])
		}

		step2 := mgr.AddStep(ShadowStep{
			Type:    "click",
			Payload: map[string]interface{}{"target": "submit-button"},
		})
		if step2.StepIndex != 2 {
			t.Errorf("expected stepIndex 2, got %d", step2.StepIndex)
		}

		st := mgr.GetState()
		if len(st.Steps) != 2 {
			t.Errorf("expected 2 steps in state, got %d", len(st.Steps))
		}
	})
}
