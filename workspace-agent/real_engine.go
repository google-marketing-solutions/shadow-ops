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
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/playwright-community/playwright-go"
)

// RealAgentEngineOptions contains configuration options for RealAgentEngine.
type RealAgentEngineOptions struct {
	WorkspaceID string
	CDPURL      string
	UserDataDir string
	Headless    bool
	ChromePath  string
	ScriptsDir  string
}

// RealAgentEngine implements BrowserEngine backed by Playwright-Go and Chromium.
type RealAgentEngine struct {
	mu           sync.RWMutex
	stateMgr     *StateManager
	pw           *playwright.Playwright
	browser      playwright.Browser
	context      playwright.BrowserContext
	page         playwright.Page
	currentCmd   *exec.Cmd
	cmdCancel    context.CancelFunc
	shadowActive bool
	steps        []ShadowStep
	options      RealAgentEngineOptions
}

// NewRealAgentEngine creates and initializes a Playwright-Go backed RealAgentEngine.
func NewRealAgentEngine(mgr *StateManager, opts RealAgentEngineOptions) (*RealAgentEngine, error) {
	if opts.UserDataDir == "" {
		opts.UserDataDir = filepath.Join(os.TempDir(), "shadowops_pw_"+opts.WorkspaceID)
	}
	if opts.ScriptsDir == "" {
		opts.ScriptsDir = "scripts"
	}

	engine := &RealAgentEngine{
		stateMgr: mgr,
		options:  opts,
		steps:    make([]ShadowStep, 0),
	}

	pw, err := playwright.Run()
	if err != nil {
		_ = playwright.Install()
		pw, err = playwright.Run()
		if err != nil {
			return nil, fmt.Errorf("failed to start playwright: %w", err)
		}
	}
	engine.pw = pw

	if opts.UserDataDir == "" {
		opts.UserDataDir = filepath.Join(os.TempDir(), fmt.Sprintf("shadowops-user-data-%s", opts.WorkspaceID))
	}
	_ = os.MkdirAll(opts.UserDataDir, 0755)

	cdpPort := 9222
	if envPort := os.Getenv("AGENT_PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil && p > 0 {
			cdpPort = p + 1000
		}
	}
	if envCDP := os.Getenv("CHROME_CDP_PORT"); envCDP != "" {
		if p, err := strconv.Atoi(envCDP); err == nil && p > 0 {
			cdpPort = p
		}
	}
	_ = os.Setenv("CHROME_CDP_URL", fmt.Sprintf("http://127.0.0.1:%d", cdpPort))

	launchArgs := []string{
		fmt.Sprintf("--remote-debugging-port=%d", cdpPort),
		"--remote-debugging-address=0.0.0.0",
		"--no-sandbox",
		"--disable-setuid-sandbox",
		"--disable-dev-shm-usage",
		"--disable-blink-features=AutomationControlled",
	}

	launchOpts := playwright.BrowserTypeLaunchPersistentContextOptions{
		Headless: playwright.Bool(opts.Headless),
		Args:     launchArgs,
		Viewport: &playwright.Size{
			Width:  1280,
			Height: 720,
		},
		IgnoreDefaultArgs: []string{"--enable-automation"},
	}
	if opts.ChromePath != "" {
		launchOpts.ExecutablePath = playwright.String(opts.ChromePath)
	}

	browserCtx, err := pw.Chromium.LaunchPersistentContext(opts.UserDataDir, launchOpts)
	if err != nil {
		_ = pw.Stop()
		return nil, fmt.Errorf("failed to launch chromium persistent context: %w", err)
	}
	engine.context = browserCtx

	// Inject stealth script
	stealthPath := filepath.Join(opts.ScriptsDir, "stealth_init.js")
	if stealthContent, err := os.ReadFile(stealthPath); err == nil {
		_ = browserCtx.AddInitScript(playwright.Script{Content: playwright.String(string(stealthContent))})
	}

	// Inject dom recorder, active element, element from point scripts
	for _, sname := range []string{"active_element.js", "element_from_point.js", "dom_recorder.js"} {
		spath := filepath.Join(opts.ScriptsDir, sname)
		if content, err := os.ReadFile(spath); err == nil {
			_ = browserCtx.AddInitScript(playwright.Script{Content: playwright.String(string(content))})
		}
	}

	attachPageListeners := func(p playwright.Page) {
		p.On("framenavigated", func(frame playwright.Frame) {
			if frame == p.MainFrame() {
				url := frame.URL()
				engine.stateMgr.UpdateState(func(s *AgentState) {
					s.CurrentURL = url
				})
			}
		})
		p.On("close", func(closedPage playwright.Page) {
			engine.mu.Lock()
			defer engine.mu.Unlock()
			pages := engine.context.Pages()
			if len(pages) > 0 {
				engine.page = pages[len(pages)-1]
				engine.stateMgr.UpdateState(func(s *AgentState) {
					s.CurrentURL = engine.page.URL()
				})
			}
		})
	}

	browserCtx.On("page", func(newPage playwright.Page) {
		engine.mu.Lock()
		engine.page = newPage
		engine.mu.Unlock()
		engine.stateMgr.UpdateState(func(s *AgentState) {
			s.CurrentURL = newPage.URL()
		})
		attachPageListeners(newPage)
	})

	pages := browserCtx.Pages()
	var page playwright.Page
	if len(pages) > 0 {
		page = pages[0]
	} else {
		newPage, err := browserCtx.NewPage()
		if err != nil {
			_ = browserCtx.Close()
			_ = pw.Stop()
			return nil, fmt.Errorf("failed to create initial page: %w", err)
		}
		page = newPage
	}
	engine.page = page
	attachPageListeners(page)

	// Navigate to initial URL (Google sign-in) in background so agent listens immediately
	go func() {
		initialURL := "https://accounts.google.com/ServiceLogin/signinchooser"
		_, _ = page.Goto(initialURL, playwright.PageGotoOptions{
			WaitUntil: playwright.WaitUntilStateDomcontentloaded,
			Timeout:   playwright.Float(20000),
		})
		engine.stateMgr.UpdateState(func(s *AgentState) {
			s.CurrentURL = page.URL()
		})
	}()

	return engine, nil
}

func (e *RealAgentEngine) getActivePage() playwright.Page {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.context != nil {
		pages := e.context.Pages()
		if len(pages) > 0 {
			e.page = pages[len(pages)-1]
			return e.page
		}
	}
	if e.page != nil && !e.page.IsClosed() {
		return e.page
	}
	return nil
}

// Navigate navigates the active page to target URL.
func (e *RealAgentEngine) Navigate(ctx context.Context, in NavigateInput) (NavigateOutput, error) {
	page := e.getActivePage()
	if page == nil {
		return NavigateOutput{}, fmt.Errorf("browser page is unavailable")
	}

	e.mu.RLock()
	isRecording := e.shadowActive
	e.mu.RUnlock()

	waitUntil := playwright.WaitUntilStateDomcontentloaded
	if in.WaitUntil == "load" {
		waitUntil = playwright.WaitUntilStateLoad
	} else if in.WaitUntil == "networkidle" {
		waitUntil = playwright.WaitUntilStateNetworkidle
	}

	timeout := 20000.0
	if in.TimeoutMS > 0 {
		timeout = float64(in.TimeoutMS)
	}

	_, err := page.Goto(in.URL, playwright.PageGotoOptions{
		WaitUntil: waitUntil,
		Timeout:   playwright.Float(timeout),
	})
	if err != nil {
		return NavigateOutput{}, fmt.Errorf("navigation failed: %w", err)
	}

	currentURL := page.URL()
	e.stateMgr.UpdateState(func(s *AgentState) {
		s.CurrentURL = currentURL
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
			Payload: map[string]interface{}{"url": currentURL},
		})
		e.mu.Lock()
		e.steps = append(e.steps, step)
		e.mu.Unlock()
	}

	return NavigateOutput{CurrentURL: currentURL, Success: true}, nil
}

// HandleInput dispatches mouse, keyboard, wheel, or viewport resize to Playwright page.
func (e *RealAgentEngine) HandleInput(ctx context.Context, in InputPayload) error {
	page := e.getActivePage()
	if page == nil {
		return fmt.Errorf("browser page is unavailable")
	}

	action := in.Action
	if action == "" {
		action = in.Event
	}

	switch in.Type {
	case "viewport.resize", "resize":
		if in.Width > 0 && in.Height > 0 {
			w := max(800, min(1600, in.Width))
			h := max(520, min(1000, in.Height))
			e.stateMgr.UpdateState(func(s *AgentState) {
				s.ViewportWidth = w
				s.ViewportHeight = h
			})
			_ = page.SetViewportSize(w, h)
		}

	case "input.mouse", "mouse":
		clickCount := in.ClickCount
		if clickCount < 1 {
			clickCount = 1
		}
		btn := playwright.MouseButtonLeft
		if in.Button == "right" || in.Button == "secondary" {
			btn = playwright.MouseButtonRight
		} else if in.Button == "middle" {
			btn = playwright.MouseButtonMiddle
		}

		e.stateMgr.UpdateState(func(s *AgentState) {
			s.CursorX = int(in.X)
			s.CursorY = int(in.Y)
			if action == "down" {
				s.MouseDown = true
			} else if action == "up" || action == "click" {
				s.MouseDown = false
			}
		})

		switch action {
		case "move", "":
			_ = page.Mouse().Move(in.X, in.Y)
		case "down":
			_ = page.Mouse().Move(in.X, in.Y)
			_ = page.Mouse().Down(playwright.MouseDownOptions{
				Button:     btn,
				ClickCount: playwright.Int(clickCount),
			})
		case "up":
			_ = page.Mouse().Move(in.X, in.Y)
			_ = page.Mouse().Up(playwright.MouseUpOptions{
				Button:     btn,
				ClickCount: playwright.Int(clickCount),
			})
		case "click":
			_ = page.Mouse().Click(in.X, in.Y, playwright.MouseClickOptions{
				Button:     btn,
				ClickCount: playwright.Int(clickCount),
			})
		}

	case "input.wheel", "wheel":
		_ = page.Mouse().Wheel(in.DeltaX, in.DeltaY)

	case "input.key", "keyboard":
		if !(action == "" || action == "down" || action == "keydown" || action == "press") {
			return nil
		}
		if in.Key == "" {
			return nil
		}
		pwKey := toPlaywrightKeyName(in.Key)
		if len(in.Key) == 1 && in.Key != "\r" && in.Key != "\n" {
			_ = page.Keyboard().Type(in.Key, playwright.KeyboardTypeOptions{Delay: playwright.Float(10)})
		} else {
			if pwKey == "" {
				pwKey = in.Code
			}
			if pwKey != "" {
				_ = page.Keyboard().Press(pwKey)
			}
		}
	}

	return nil
}

// StartShadow initiates shadow recording.
func (e *RealAgentEngine) StartShadow(ctx context.Context) error {
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
func (e *RealAgentEngine) EndShadow(ctx context.Context) ([]ShadowStep, error) {
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

// RenderFrame captures a real JPEG screenshot from the active Playwright page.
func (e *RealAgentEngine) RenderFrame(ctx context.Context) (*FrameEvent, error) {
	page := e.getActivePage()
	if page == nil {
		return nil, fmt.Errorf("page unavailable")
	}

	jpegBytes, err := page.Screenshot(playwright.PageScreenshotOptions{
		Type:     playwright.ScreenshotTypeJpeg,
		Quality:  playwright.Int(65),
		FullPage: playwright.Bool(false),
	})
	if err != nil {
		return nil, err
	}

	st := e.stateMgr.GetState()
	return &FrameEvent{
		Type:   "frame",
		Seq:    st.FrameSeq + 1,
		TS:     time.Now().Unix(),
		Format: "jpeg",
		Width:  st.ViewportWidth,
		Height: st.ViewportHeight,
		Data:   base64.StdEncoding.EncodeToString(jpegBytes),
	}, nil
}

// RunScript executes an uploaded Python script using the CDP runner subprocess.
func (e *RealAgentEngine) RunScript(ctx context.Context, in ScriptRunInput, onLog func(ScriptLogEntry)) (ScriptRunOutput, error) {
	tmpDir, err := os.MkdirTemp("", "shadowops_run_"+in.RunID)
	if err != nil {
		return ScriptRunOutput{RunID: in.RunID, Status: "failed", Error: err.Error()}, err
	}
	defer os.RemoveAll(tmpDir)

	userScriptPath := filepath.Join(tmpDir, in.Filename)
	if in.Filename == "" {
		userScriptPath = filepath.Join(tmpDir, "script.py")
	}
	runnerScriptPath := filepath.Join(tmpDir, "runner.py")

	if err := os.WriteFile(userScriptPath, []byte(in.Script), 0644); err != nil {
		return ScriptRunOutput{RunID: in.RunID, Status: "failed", Error: err.Error()}, err
	}
	if err := os.WriteFile(runnerScriptPath, []byte(PythonRunnerTemplate), 0644); err != nil {
		return ScriptRunOutput{RunID: in.RunID, Status: "failed", Error: err.Error()}, err
	}

	inputsBytes, _ := json.Marshal(in.Inputs)

	cmdCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(cmdCtx, "python3", runnerScriptPath, userScriptPath, string(inputsBytes))
	cmd.Env = append(os.Environ(), "PYTHONUNBUFFERED=1")

	e.mu.Lock()
	e.currentCmd = cmd
	e.cmdCancel = cancel
	e.mu.Unlock()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return ScriptRunOutput{RunID: in.RunID, Status: "failed", Error: err.Error()}, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return ScriptRunOutput{RunID: in.RunID, Status: "failed", Error: err.Error()}, err
	}

	if err := cmd.Start(); err != nil {
		return ScriptRunOutput{RunID: in.RunID, Status: "failed", Error: err.Error()}, err
	}

	var logsMu sync.Mutex
	var collectedLogs []ScriptLogEntry
	var resultMu sync.Mutex
	var parsedResult map[string]interface{}

	handleLine := func(line string) {
		line = strings.TrimSpace(line)
		if line == "" {
			return
		}
		if strings.HasPrefix(line, "[SCRIPT_RESULT] ") {
			payload := strings.TrimPrefix(line, "[SCRIPT_RESULT] ")
			resultMu.Lock()
			var res map[string]interface{}
			if err := json.Unmarshal([]byte(payload), &res); err == nil {
				parsedResult = res
			}
			resultMu.Unlock()
			return
		}

		level := "INFO"
		msg := line
		if strings.HasPrefix(line, "[SCRIPT_LOG] ") {
			parts := strings.SplitN(strings.TrimPrefix(line, "[SCRIPT_LOG] "), "|", 2)
			if len(parts) == 2 {
				level = parts[0]
				msg = parts[1]
			}
		}

		entry := ScriptLogEntry{
			Timestamp: NowISO(),
			Level:     level,
			Message:   msg,
		}

		logsMu.Lock()
		collectedLogs = append(collectedLogs, entry)
		logsMu.Unlock()

		if onLog != nil {
			onLog(entry)
		}
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			handleLine(scanner.Text())
		}
	}()

	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			handleLine(scanner.Text())
		}
	}()

	wg.Wait()
	cmdErr := cmd.Wait()

	resultMu.Lock()
	finalResult := parsedResult
	resultMu.Unlock()

	status := "succeeded"
	errMsg := ""
	if cmdErr != nil {
		status = "failed"
		errMsg = cmdErr.Error()
	}

	if finalResult != nil {
		if st, ok := finalResult["status"].(string); ok {
			status = st
		}
		if em, ok := finalResult["error"].(string); ok && em != "" {
			errMsg = em
		}
	}

	return ScriptRunOutput{
		RunID:  in.RunID,
		Status: status,
		Result: finalResult,
		Error:  errMsg,
		Logs:   collectedLogs,
	}, nil
}

// CancelScript terminates the active python runner subprocess if executing.
func (e *RealAgentEngine) CancelScript(ctx context.Context, runID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.cmdCancel != nil {
		e.cmdCancel()
	}
	if e.currentCmd != nil && e.currentCmd.Process != nil {
		_ = e.currentCmd.Process.Kill()
	}
	return nil
}

// Close gracefully closes the browser context and stops Playwright.
func (e *RealAgentEngine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.context != nil {
		_ = e.context.Close()
	}
	if e.browser != nil {
		_ = e.browser.Close()
	}
	if e.pw != nil {
		_ = e.pw.Stop()
	}
	log.Println("RealAgentEngine closed successfully")
	return nil
}

func toPlaywrightKeyName(key string) string {
	switch key {
	case " ", "Spacebar":
		return "Space"
	case "Esc":
		return "Escape"
	}
	return key
}
