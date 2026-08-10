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
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config holds all configurable runtime settings for the ShadowOps backend.
type Config struct {
	RepoRoot           string
	Port               string
	BindAddr           string
	DataDir            string
	StaticDir          string
	HostAgentURL       string
	WorkspaceAgentHost string
	WorkspaceAgentPort int
	WorkspaceImageName string
	WorkspacePodImage  string
	K8sNamespace       string
	PodPoolLimit       int
	PodCPURequest      string
	PodMemoryRequest   string
	PodCPULimit        string
	PodMemoryLimit     string
	BrowserHeadless    string
	GKEEndpoint        string
	GKECAData          string
	DefaultTargetURL   string
	GeminiAPIEndpoint  string
	GeminiModel        string
	ShadowOpsEnabled   bool
}

// LoadConfig initializes a Config struct populated from environment variables with sensible defaults.
func LoadConfig(repoRoot string) *Config {
	repoRootVal := repoRoot
	if repoRootVal == "" {
		if envRoot := os.Getenv("REPO_ROOT"); envRoot != "" {
			repoRootVal = envRoot
		} else {
			repoRootVal = "."
		}
	}

	port := getEnv("PORT", "8080")
	if customPort := os.Getenv("BACKEND_PORT"); customPort != "" {
		port = customPort
	}

	bindAddr := os.Getenv("BIND_ADDR")
	if bindAddr == "" {
		if strings.HasPrefix(port, ":") {
			bindAddr = port
		} else {
			bindAddr = ":" + port
		}
	}

	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = filepath.Join(repoRootVal, "data")
	}

	staticDir := os.Getenv("STATIC_DIR")
	if staticDir == "" {
		candidates := []string{
			filepath.Join(repoRootVal, "frontend", "dist", "frontend", "browser"),
			filepath.Join(repoRootVal, "dist", "frontend", "browser"),
			filepath.Join(repoRootVal, "frontend", "dist", "frontend"),
			filepath.Join(repoRootVal, "public"),
		}
		for _, c := range candidates {
			if info, err := os.Stat(c); err == nil && info.IsDir() {
				staticDir = c
				break
			}
		}
	}

	hostAgentURL := os.Getenv("HOST_AGENT_URL")
	if hostAgentURL == "" {
		hostAgentAddr := getEnv("HOST_AGENT_ADDR", getEnv("HOST_AGENT_IP", "127.0.0.1:8081"))
		if !strings.HasPrefix(hostAgentAddr, "http://") && !strings.HasPrefix(hostAgentAddr, "https://") {
			hostAgentURL = "http://" + hostAgentAddr
		} else {
			hostAgentURL = hostAgentAddr
		}
	}
	hostAgentURL = strings.TrimRight(hostAgentURL, "/")

	wsHost := strings.TrimSpace(os.Getenv("WORKSPACE_AGENT_HOST"))
	if wsHost == "" {
		wsHost = "127.0.0.1"
		if _, err := net.LookupHost("host.docker.internal"); err == nil {
			wsHost = "host.docker.internal"
		}
	}
	wsPort := 8765
	if portStr := os.Getenv("WORKSPACE_AGENT_PORT"); portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil && p > 0 {
			wsPort = p
		}
	}

	wsImage := getEnv("WORKSPACE_IMAGE_NAME", "shadowops-workspace-agent:dev")
	podImage := getEnv("WORKSPACE_POD_IMAGE", getEnv("WORKSPACE_CONTAINER_IMAGE", "shadowops-workspace-agent:latest"))
	k8sNamespace := getEnv("K8S_NAMESPACE", getEnv("GKE_NAMESPACE", "default"))

	podPoolLimit := 10
	if limitStr := os.Getenv("POD_POOL_LIMIT"); limitStr != "" {
		if val, err := strconv.Atoi(limitStr); err == nil && val > 0 {
			podPoolLimit = val
		}
	}

	podCPUReq := getEnv("POD_CPU_REQUEST", "200m")
	podMemReq := getEnv("POD_MEMORY_REQUEST", "512Mi")
	podCPULim := os.Getenv("POD_CPU_LIMIT")
	podMemLim := os.Getenv("POD_MEMORY_LIMIT")
	browserHeadless := getEnv("BROWSER_HEADLESS", "false")
	gkeEndpoint := os.Getenv("GKE_ENDPOINT")
	gkeCAData := os.Getenv("GKE_CA_CERT")
	defaultTarget := getEnv("DEFAULT_TARGET_URL", "https://accounts.google.com/ServiceLogin/signinchooser")

	geminiModel := getEnv("GEMINI_MODEL", "gemini-2.5-pro")
	geminiEndpoint := os.Getenv("GEMINI_API_ENDPOINT")
	if geminiEndpoint == "" {
		geminiEndpoint = fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent", geminiModel)
	}

	shadowopsEnabled := strings.EqualFold(strings.TrimSpace(os.Getenv("SHADOWOPS_ENABLED")), "true")

	return &Config{
		RepoRoot:           repoRootVal,
		Port:               port,
		BindAddr:           bindAddr,
		DataDir:            dataDir,
		StaticDir:          staticDir,
		HostAgentURL:       hostAgentURL,
		WorkspaceAgentHost: wsHost,
		WorkspaceAgentPort: wsPort,
		WorkspaceImageName: wsImage,
		WorkspacePodImage:  podImage,
		K8sNamespace:       k8sNamespace,
		PodPoolLimit:       podPoolLimit,
		PodCPURequest:      podCPUReq,
		PodMemoryRequest:   podMemReq,
		PodCPULimit:        podCPULim,
		PodMemoryLimit:     podMemLim,
		BrowserHeadless:    browserHeadless,
		GKEEndpoint:        gkeEndpoint,
		GKECAData:          gkeCAData,
		DefaultTargetURL:   defaultTarget,
		GeminiAPIEndpoint:  geminiEndpoint,
		GeminiModel:        geminiModel,
		ShadowOpsEnabled:   shadowopsEnabled,
	}
}

// getEnv retrieves the value of the environment variable named by key, or fallback if empty.
func getEnv(key, fallback string) string {
	if val := strings.TrimSpace(os.Getenv(key)); val != "" {
		return val
	}
	return fallback
}
