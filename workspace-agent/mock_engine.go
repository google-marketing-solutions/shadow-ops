package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/color"
	"image/jpeg"
	"sync"
	"time"
)

// MockAgentEngine implements BrowserEngine for headless testing and synthetic environments.
type MockAgentEngine struct {
	mu           sync.RWMutex
	stateMgr     *StateManager
	currentURL   string
	frameSeq     int64
	shadowActive bool
	steps        []ShadowStep
}

// NewMockAgentEngine creates a new MockAgentEngine instance.
func NewMockAgentEngine(mgr *StateManager) *MockAgentEngine {
	return &MockAgentEngine{
		stateMgr:   mgr,
		currentURL: "https://accounts.google.com/ServiceLogin/signinchooser",
		steps:      make([]ShadowStep, 0),
	}
}

// Navigate sets current URL and records shadow step if recording.
func (e *MockAgentEngine) Navigate(ctx context.Context, in NavigateInput) (NavigateOutput, error) {
	e.mu.Lock()
	e.currentURL = in.URL
	isRecording := e.shadowActive
	e.mu.Unlock()

	e.stateMgr.UpdateState(func(s *AgentState) {
		s.CurrentURL = in.URL
		if isRecording {
			s.Status = "recording"
			s.LiveMessage = "Recording ON"
		} else {
			s.Status = "active"
			s.LiveMessage = "Manual Control Active"
		}
	})

	if isRecording {
		step := e.stateMgr.AddStep(ShadowStep{
			Type:    "navigate",
			Payload: map[string]interface{}{"url": in.URL},
		})
		e.mu.Lock()
		e.steps = append(e.steps, step)
		e.mu.Unlock()
	}

	return NavigateOutput{CurrentURL: in.URL, Success: true}, nil
}

// HandleInput processes synthetic mouse, key, or resize events.
func (e *MockAgentEngine) HandleInput(ctx context.Context, in InputPayload) error {
	action := in.Action
	if action == "" {
		action = in.Event
	}

	e.stateMgr.UpdateState(func(s *AgentState) {
		if (in.Type == "viewport.resize" || in.Type == "resize") && in.Width > 0 && in.Height > 0 {
			s.ViewportWidth = in.Width
			s.ViewportHeight = in.Height
		} else if in.Type == "input.mouse" || in.Type == "mouse" {
			s.CursorX = int(in.X)
			s.CursorY = int(in.Y)
			if action == "down" {
				s.MouseDown = true
			} else if action == "up" || action == "click" {
				s.MouseDown = false
			}
		}
	})
	return nil
}

// StartShadow initiates shadow step recording mode.
func (e *MockAgentEngine) StartShadow(ctx context.Context) error {
	e.mu.Lock()
	e.shadowActive = true
	e.steps = make([]ShadowStep, 0)
	e.mu.Unlock()

	e.stateMgr.UpdateState(func(s *AgentState) {
		s.RecordingState = "on"
		s.Status = "recording"
		s.RecordingStartedAt = NowISO()
		s.Steps = make([]ShadowStep, 0)
	})
	return nil
}

// EndShadow stops shadow recording and returns captured steps.
func (e *MockAgentEngine) EndShadow(ctx context.Context) ([]ShadowStep, error) {
	e.mu.Lock()
	e.shadowActive = false
	res := make([]ShadowStep, len(e.steps))
	copy(res, e.steps)
	e.mu.Unlock()

	e.stateMgr.UpdateState(func(s *AgentState) {
		s.RecordingState = "off"
		s.Status = "active"
	})
	return res, nil
}

// RenderFrame generates a synthetic JPEG frame.
func (e *MockAgentEngine) RenderFrame(ctx context.Context) (*FrameEvent, error) {
	e.mu.Lock()
	e.frameSeq++
	seq := e.frameSeq
	e.mu.Unlock()

	st := e.stateMgr.GetState()
	w, h := st.ViewportWidth, st.ViewportHeight
	if w <= 0 {
		w = 1280
	}
	if h <= 0 {
		h = 720
	}

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	bgColor := color.RGBA{R: 15, G: 25, B: 35, A: 255}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, bgColor)
		}
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 65}); err != nil {
		return nil, err
	}

	return &FrameEvent{
		Type:   "frame",
		Seq:    seq,
		TS:     time.Now().Unix(),
		Format: "jpeg",
		Width:  w,
		Height: h,
		Data:   base64.StdEncoding.EncodeToString(buf.Bytes()),
	}, nil
}

// RunScript simulates script execution for testing.
func (e *MockAgentEngine) RunScript(ctx context.Context, in ScriptRunInput, onLog func(ScriptLogEntry)) (ScriptRunOutput, error) {
	if onLog != nil {
		onLog(ScriptLogEntry{
			Timestamp: NowISO(),
			Level:     "INFO",
			Message:   "Starting simulated script execution: " + in.Filename,
		})
	}
	time.Sleep(50 * time.Millisecond)

	out := ScriptRunOutput{
		RunID:  in.RunID,
		Status: "succeeded",
		Result: map[string]interface{}{
			"status": "succeeded",
			"ok":     true,
		},
		Logs: []ScriptLogEntry{
			{Timestamp: NowISO(), Level: "INFO", Message: "Simulated run complete"},
		},
	}
	return out, nil
}

// CancelScript cancels ongoing script.
func (e *MockAgentEngine) CancelScript(ctx context.Context, runID string) error {
	return nil
}

// Close cleans up engine resources.
func (e *MockAgentEngine) Close() error {
	return nil
}
