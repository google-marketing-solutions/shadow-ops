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
	"context"
	"testing"
)

func TestMockAgentEngine(t *testing.T) {
	ctx := context.Background()
	mgr := NewStateManager("ws_mock_test")
	engine := NewMockAgentEngine(mgr)

	t.Run("Navigate updates state and records step when recording", func(t *testing.T) {
		_ = engine.StartShadow(ctx)
		navOut, err := engine.Navigate(ctx, NavigateInput{URL: "https://ads.google.com"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if navOut.CurrentURL != "https://ads.google.com" || !navOut.Success {
			t.Errorf("unexpected navigate output: %+v", navOut)
		}

		steps, err := engine.EndShadow(ctx)
		if err != nil {
			t.Fatalf("unexpected EndShadow error: %v", err)
		}
		if len(steps) != 1 {
			t.Errorf("expected 1 step, got %d", len(steps))
		}
		if steps[0].Type != "navigate" {
			t.Errorf("expected step type navigate, got %s", steps[0].Type)
		}
	})

	t.Run("RenderFrame produces valid base64 jpeg event", func(t *testing.T) {
		frame, err := engine.RenderFrame(ctx)
		if err != nil {
			t.Fatalf("unexpected error rendering frame: %v", err)
		}
		if frame.Format != "jpeg" {
			t.Errorf("expected format jpeg, got %s", frame.Format)
		}
		if len(frame.Data) == 0 {
			t.Errorf("expected non-empty base64 frame data")
		}
		if frame.Seq <= 0 {
			t.Errorf("expected positive frame sequence, got %d", frame.Seq)
		}
	})

	t.Run("RunScript produces successful output", func(t *testing.T) {
		var logs []ScriptLogEntry
		out, err := engine.RunScript(ctx, ScriptRunInput{
			RunID:    "run_1",
			Filename: "test.py",
			Script:   "print('hello')",
		}, func(l ScriptLogEntry) {
			logs = append(logs, l)
		})
		if err != nil {
			t.Fatalf("unexpected script run error: %v", err)
		}
		if out.Status != "succeeded" {
			t.Errorf("expected status succeeded, got %s", out.Status)
		}
		if len(logs) == 0 {
			t.Errorf("expected log entries received")
		}
	})
}

func TestMockAgentEngine_HandleInput(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		input    InputPayload
		validate func(t *testing.T, s AgentState)
	}{
		{
			name: "viewport resize using type viewport.resize",
			input: InputPayload{
				Type:   "viewport.resize",
				Width:  1280,
				Height: 720,
			},
			validate: func(t *testing.T, s AgentState) {
				if s.ViewportWidth != 1280 || s.ViewportHeight != 720 {
					t.Errorf("expected viewport 1280x720, got %dx%d", s.ViewportWidth, s.ViewportHeight)
				}
			},
		},
		{
			name: "viewport resize using short type resize",
			input: InputPayload{
				Type:   "resize",
				Width:  1024,
				Height: 768,
			},
			validate: func(t *testing.T, s AgentState) {
				if s.ViewportWidth != 1024 || s.ViewportHeight != 768 {
					t.Errorf("expected viewport 1024x768, got %dx%d", s.ViewportWidth, s.ViewportHeight)
				}
			},
		},
		{
			name: "mouse down using Action field",
			input: InputPayload{
				Type:   "mouse",
				Action: "down",
				X:      250,
				Y:      300,
			},
			validate: func(t *testing.T, s AgentState) {
				if s.CursorX != 250 || s.CursorY != 300 || !s.MouseDown {
					t.Errorf("expected cursor (250, 300) mouseDown=true, got (%d, %d) mouseDown=%v", s.CursorX, s.CursorY, s.MouseDown)
				}
			},
		},
		{
			name: "mouse up using Event field fallback",
			input: InputPayload{
				Type:  "input.mouse",
				Event: "up",
				X:     260,
				Y:     310,
			},
			validate: func(t *testing.T, s AgentState) {
				if s.CursorX != 260 || s.CursorY != 310 || s.MouseDown {
					t.Errorf("expected cursor (260, 310) mouseDown=false, got (%d, %d) mouseDown=%v", s.CursorX, s.CursorY, s.MouseDown)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mgr := NewStateManager("ws_mock_test_input")
			engine := NewMockAgentEngine(mgr)
			if err := engine.HandleInput(ctx, tc.input); err != nil {
				t.Fatalf("unexpected HandleInput error: %v", err)
			}
			state := mgr.GetState()
			tc.validate(t, state)
		})
	}
}
