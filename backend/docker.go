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
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"strings"
)

func (s *Server) workspaceImageName() string {
	if s != nil && s.config != nil && s.config.WorkspaceImageName != "" {
		return s.config.WorkspaceImageName
	}
	return "shadowops-workspace-agent:dev"
}

func (s *Server) ensureWorkspaceImage() error {
	s.imageBuildOnce.Do(func() {
		// If docker CLI is not installed in the backend environment (e.g. running in container),
		// the image is pre-built or managed by the host-agent daemon.
		if _, err := exec.LookPath("docker"); err != nil {
			return
		}
		if _, stderr, err := s.runCmd(s.repoRoot, "docker", "info"); err != nil {
			log.Printf("warning: docker daemon unavailable (%v: %s), skipping local workspace image build", err, strings.TrimSpace(stderr))
			return
		}
		buildDir := filepath.Join(s.repoRoot, "workspace-agent")
		imageName := s.workspaceImageName()
		if _, stderr, err := s.runCmd(buildDir, "docker", "build", "-t", imageName, "-f", "Dockerfile", "."); err != nil {
			s.imageBuildErr = fmt.Errorf("docker build failed: %v (%s)", err, strings.TrimSpace(stderr))
			return
		}
	})
	return s.imageBuildErr
}

func (s *Server) runWorkspaceContainer(workspaceID string, hostPort int) (string, error) {
	name := "shadowops-" + strings.ReplaceAll(workspaceID, "_", "-")

	reqBody := map[string]any{
		"token":          "dummy",
		"port":           hostPort,
		"containerName":  name,
		"containerImage": s.workspaceImageName(),
	}

	respBody, statusCode, err := s.callHostAgent(context.Background(), "POST", "/containers", reqBody)
	if err != nil {
		return "", fmt.Errorf("host agent call failed: %w", err)
	}
	if statusCode >= 300 {
		return "", fmt.Errorf("host agent returned status %d: %s", statusCode, string(respBody))
	}

	var resp struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return "", fmt.Errorf("failed to parse host agent response: %w", err)
	}

	return resp.ID, nil
}

func (s *Server) removeWorkspaceContainer(ws *Workspace) error {
	ws.mu.RLock()
	containerID := ws.DockerContainerID
	ws.mu.RUnlock()
	if strings.TrimSpace(containerID) == "" {
		return nil
	}

	respBody, statusCode, err := s.callHostAgent(context.Background(), "DELETE", "/containers/"+containerID, nil)
	if err != nil {
		return fmt.Errorf("host agent call failed: %w", err)
	}
	if statusCode >= 300 {
		return fmt.Errorf("host agent returned status %d: %s", statusCode, string(respBody))
	}

	return nil
}

func (s *Server) runCmd(dir string, name string, args ...string) (string, string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}
