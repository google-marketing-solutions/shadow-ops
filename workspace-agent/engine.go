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
)

// NavigateInput holds parameters for page navigation.
type NavigateInput struct {
	URL       string `json:"url"`
	WaitUntil string `json:"waitUntil,omitempty"`
	TimeoutMS int    `json:"timeoutMs,omitempty"`
}

// NavigateOutput holds the result of page navigation.
type NavigateOutput struct {
	CurrentURL string `json:"currentUrl"`
	Success    bool   `json:"success"`
}

// InputPayload encapsulates browser mouse, wheel, keyboard, or viewport inputs.
type InputPayload struct {
	Type       string  `json:"type"`
	Action     string  `json:"action,omitempty"`
	Event      string  `json:"event,omitempty"`
	URL        string  `json:"url,omitempty"`
	X          float64 `json:"x,omitempty"`
	Y          float64 `json:"y,omitempty"`
	Button     string  `json:"button,omitempty"`
	Buttons    int     `json:"buttons,omitempty"`
	ClickCount int     `json:"clickCount,omitempty"`
	DeltaX     float64 `json:"deltaX,omitempty"`
	DeltaY     float64 `json:"deltaY,omitempty"`
	Key        string  `json:"key,omitempty"`
	Code       string  `json:"code,omitempty"`
	Width      int     `json:"width,omitempty"`
	Height     int     `json:"height,omitempty"`
}

// ScriptRunInput specifies the automation script code and execution parameters.
type ScriptRunInput struct {
	RunID    string                 `json:"runId"`
	Filename string                 `json:"filename,omitempty"`
	Script   string                 `json:"script"`
	Inputs   map[string]interface{} `json:"inputs,omitempty"`
}

// ScriptRunOutput represents the outcome of script execution.
type ScriptRunOutput struct {
	RunID  string                 `json:"runId"`
	Status string                 `json:"status"` // succeeded, failed, cancelled
	Result map[string]interface{} `json:"result,omitempty"`
	Error  string                 `json:"error,omitempty"`
	Logs   []ScriptLogEntry       `json:"logs,omitempty"`
}

// BrowserEngine defines the interface for browser lifecycle, rendering, and interaction.
type BrowserEngine interface {
	Navigate(ctx context.Context, in NavigateInput) (NavigateOutput, error)
	HandleInput(ctx context.Context, in InputPayload) error
	StartShadow(ctx context.Context) error
	EndShadow(ctx context.Context) ([]ShadowStep, error)
	RenderFrame(ctx context.Context) (*FrameEvent, error)
	RunScript(ctx context.Context, in ScriptRunInput, onLog func(ScriptLogEntry)) (ScriptRunOutput, error)
	CancelScript(ctx context.Context, runID string) error
	Close() error
}
