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
