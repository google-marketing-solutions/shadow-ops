package main

import (
	"encoding/json"
	"sync"
	"time"
)

// NowISO returns the current time formatted as ISO-8601 UTC.
func NowISO() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// ScriptLogEntry represents an individual log line emitted during script execution.
type ScriptLogEntry struct {
	Timestamp string `json:"ts"`
	Level     string `json:"level"`
	Message   string `json:"message"`
}

// ScriptRunState captures the status and outputs of an automation script execution.
type ScriptRunState struct {
	RunID           string                 `json:"runId"`
	Status          string                 `json:"status"` // idle, running, cancelling, succeeded, failed, cancelled
	Filename        string                 `json:"filename,omitempty"`
	Inputs          map[string]interface{} `json:"inputs,omitempty"`
	StartedAt       string                 `json:"startedAt,omitempty"`
	FinishedAt      string                 `json:"finishedAt,omitempty"`
	Error           string                 `json:"error,omitempty"`
	Traceback       string                 `json:"traceback,omitempty"`
	Result          map[string]interface{} `json:"result,omitempty"`
	Logs            []ScriptLogEntry       `json:"logs,omitempty"`
	CancelRequested bool                   `json:"cancelRequested,omitempty"`
}

// ShadowStep represents a recorded browser user interaction step.
type ShadowStep struct {
	StepIndex int                    `json:"stepIndex"`
	Type      string                 `json:"type"`
	Payload   map[string]interface{} `json:"payload"`
	Meta      map[string]interface{} `json:"meta,omitempty"`
}

// FrameEvent represents a screencast frame payload broadcast over WebSockets.
type FrameEvent struct {
	Type   string `json:"type"`
	Seq    int64  `json:"seq"`
	TS     int64  `json:"ts"`
	Format string `json:"format"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Data   string `json:"data"`
}

// AgentState represents the overall state of the workspace agent session.
type AgentState struct {
	WorkspaceID        string         `json:"workspaceId"`
	Status             string         `json:"status"`         // active, recording, killed
	RecordingState     string         `json:"recordingState"` // off, on
	CurrentURL         string         `json:"currentUrl"`
	ViewportWidth      int            `json:"viewportWidth"`
	ViewportHeight     int            `json:"viewportHeight"`
	CursorX            int            `json:"cursorX"`
	CursorY            int            `json:"cursorY"`
	MouseDown          bool           `json:"mouseDown"`
	FrameSeq           int64          `json:"frameSeq"`
	SessionStartedAt   string         `json:"sessionStartedAt"`
	RecordingStartedAt string         `json:"recordingStartedAt,omitempty"`
	LiveMessage        string         `json:"liveMessage,omitempty"`
	HelperMessage      string         `json:"helperMessage,omitempty"`
	Steps              []ShadowStep   `json:"steps"`
	ScriptRun          ScriptRunState `json:"scriptRun"`
	LastFillSignature  string         `json:"-"`
	LastClickSignature string         `json:"-"`
	LastClickAt        int64          `json:"-"`
	LastScrollAt       int64          `json:"-"`
}

// NewInitialAgentState creates an AgentState with sensible default values.
func NewInitialAgentState(workspaceID string) AgentState {
	if workspaceID == "" {
		workspaceID = "ws_local"
	}
	return AgentState{
		WorkspaceID:      workspaceID,
		Status:           "active",
		RecordingState:   "off",
		CurrentURL:       "https://accounts.google.com/ServiceLogin/signinchooser",
		ViewportWidth:    1280,
		ViewportHeight:   720,
		CursorX:          520,
		CursorY:          290,
		SessionStartedAt: NowISO(),
		LiveMessage:      "Manual Control Active",
		HelperMessage:    "ShadowOps is observing your inputs",
		Steps:            make([]ShadowStep, 0),
		ScriptRun: ScriptRunState{
			Status: "idle",
			Logs:   make([]ScriptLogEntry, 0),
		},
	}
}

// StateManager provides thread-safe access to AgentState.
type StateManager struct {
	mu    sync.RWMutex
	state AgentState
}

// NewStateManager constructs a new thread-safe StateManager.
func NewStateManager(workspaceID string) *StateManager {
	return &StateManager{
		state: NewInitialAgentState(workspaceID),
	}
}

// GetState returns a copy of the current agent state.
func (m *StateManager) GetState() AgentState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cloneStateLocked()
}

// UpdateState runs a mutating function safely with write lock.
func (m *StateManager) UpdateState(fn func(*AgentState)) AgentState {
	m.mu.Lock()
	defer m.mu.Unlock()
	fn(&m.state)
	return m.cloneStateLocked()
}

// AddStep appends a recorded step and returns the created ShadowStep.
func (m *StateManager) AddStep(step ShadowStep) ShadowStep {
	m.mu.Lock()
	defer m.mu.Unlock()
	step.StepIndex = len(m.state.Steps) + 1
	if step.Meta == nil {
		step.Meta = make(map[string]interface{})
	}
	if _, ok := step.Meta["capturedAt"]; !ok {
		step.Meta["capturedAt"] = NowISO()
	}
	if _, ok := step.Meta["url"]; !ok {
		step.Meta["url"] = m.state.CurrentURL
	}
	m.state.Steps = append(m.state.Steps, step)
	return step
}

func (m *StateManager) cloneStateLocked() AgentState {
	s := m.state
	if len(m.state.Steps) > 0 {
		s.Steps = make([]ShadowStep, len(m.state.Steps))
		copy(s.Steps, m.state.Steps)
	} else {
		s.Steps = make([]ShadowStep, 0)
	}
	if len(m.state.ScriptRun.Logs) > 0 {
		s.ScriptRun.Logs = make([]ScriptLogEntry, len(m.state.ScriptRun.Logs))
		copy(s.ScriptRun.Logs, m.state.ScriptRun.Logs)
	} else {
		s.ScriptRun.Logs = make([]ScriptLogEntry, 0)
	}
	return s
}

// ToJSON safely marshals any struct to raw JSON bytes.
func ToJSON(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}
