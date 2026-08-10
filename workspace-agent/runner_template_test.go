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
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPythonRunnerTemplate_SyntaxValidation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "runner_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	runnerFile := filepath.Join(tmpDir, "runner.py")
	if err := os.WriteFile(runnerFile, []byte(PythonRunnerTemplate), 0644); err != nil {
		t.Fatalf("failed to write runner file: %v", err)
	}

	// Verify Python syntax compilation
	cmd := exec.Command("python3", "-m", "py_compile", runnerFile)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("python syntax check failed on runner template: %v, output: %s", err, string(out))
	}
}

func TestSampleScript_TurnOnECMultiple_StructureAndCompilation(t *testing.T) {
	samplePath := filepath.Join("..", "..", "sample_scripts", "turn_on_ec_multiple.py")
	content, err := os.ReadFile(samplePath)
	if err != nil {
		t.Skipf("sample script not present at %s, skipping test: %v", samplePath, err)
		return
	}

	tmpDir, err := os.MkdirTemp("", "sample_script_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	scriptFile := filepath.Join(tmpDir, "turn_on_ec_multiple.py")
	if err := os.WriteFile(scriptFile, content, 0644); err != nil {
		t.Fatalf("failed to write script: %v", err)
	}

	// Verify Python syntax
	cmd := exec.Command("python3", "-m", "py_compile", scriptFile)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("syntax compilation failed on turn_on_ec_multiple.py: %v, output: %s", err, string(out))
	}

	// Verify run definition
	inspectScript := `
import ast

with open('` + scriptFile + `', 'r') as f:
    tree = ast.parse(f.read())

functions = [node.name for node in tree.body if isinstance(node, ast.AsyncFunctionDef)]
assert 'run' in functions, "Missing async def run function"
assert 'ensure_google_ads_loaded' in functions, "Missing ensure_google_ads_loaded"
assert ('navigate_to_mcc' in functions or 'ensure_mcc_selected' in functions), "Missing MCC navigation function"
print("OK")
`
	cmdInspect := exec.Command("python3", "-c", inspectScript)
	if out, err := cmdInspect.CombinedOutput(); err != nil {
		t.Fatalf("inspection failed on turn_on_ec_multiple.py: %v, output: %s", err, string(out))
	}
}

func TestScriptExecutionOutputParsing(t *testing.T) {
	rawLogs := []string{
		"[SCRIPT_LOG] INFO|Starting account processing...",
		"[SCRIPT_LOG] WARN|Selector check warning",
		"[SCRIPT_LOG] STDOUT|Standard output line",
		"[SCRIPT_RESULT] {\"status\": \"succeeded\", \"before\": \"off\", \"after\": \"on\", \"processed_accounts\": [\"8056520078\"]}",
	}

	var collectedLogs []ScriptLogEntry
	var result map[string]interface{}

	for _, line := range rawLogs {
		if strings.HasPrefix(line, "[SCRIPT_RESULT] ") {
			payload := strings.TrimPrefix(line, "[SCRIPT_RESULT] ")
			_ = json.Unmarshal([]byte(payload), &result)
			continue
		}
		if strings.HasPrefix(line, "[SCRIPT_LOG] ") {
			parts := strings.SplitN(strings.TrimPrefix(line, "[SCRIPT_LOG] "), "|", 2)
			if len(parts) == 2 {
				collectedLogs = append(collectedLogs, ScriptLogEntry{
					Timestamp: NowISO(),
					Level:     parts[0],
					Message:   parts[1],
				})
			}
		}
	}

	if len(collectedLogs) != 3 {
		t.Fatalf("expected 3 log entries, got %d", len(collectedLogs))
	}
	if collectedLogs[0].Level != "INFO" || collectedLogs[1].Level != "WARN" {
		t.Errorf("unexpected log levels: %+v", collectedLogs)
	}
	if result["status"] != "succeeded" {
		t.Errorf("expected succeeded status, got %v", result["status"])
	}
	if result["before"] != "off" || result["after"] != "on" {
		t.Errorf("unexpected result values: %+v", result)
	}
}
