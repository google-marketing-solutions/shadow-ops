package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_Defaults(t *testing.T) {
	// Clear any existing env vars that might interfere
	vars := []string{
		"REPO_ROOT", "PORT", "BACKEND_PORT", "BIND_ADDR", "DATA_DIR", "STATIC_DIR",
		"HOST_AGENT_URL", "HOST_AGENT_ADDR", "HOST_AGENT_IP",
		"WORKSPACE_AGENT_HOST", "WORKSPACE_AGENT_PORT", "WORKSPACE_IMAGE_NAME", "WORKSPACE_POD_IMAGE",
		"K8S_NAMESPACE", "GKE_NAMESPACE", "POD_POOL_LIMIT", "POD_CPU_REQUEST",
		"POD_MEMORY_REQUEST", "POD_CPU_LIMIT", "POD_MEMORY_LIMIT", "BROWSER_HEADLESS",
		"GKE_ENDPOINT", "GKE_CA_CERT", "DEFAULT_TARGET_URL", "GEMINI_API_ENDPOINT",
		"GEMINI_MODEL", "SHADOWOPS_ENABLED",
	}
	for _, v := range vars {
		os.Unsetenv(v)
	}

	repoRoot := t.TempDir()
	cfg := LoadConfig(repoRoot)

	if cfg.RepoRoot != repoRoot {
		t.Errorf("expected default RepoRoot %s, got %s", repoRoot, cfg.RepoRoot)
	}
	if cfg.Port != "8080" {
		t.Errorf("expected default Port 8080, got %s", cfg.Port)
	}
	if cfg.BindAddr != ":8080" {
		t.Errorf("expected default BindAddr :8080, got %s", cfg.BindAddr)
	}
	if cfg.DataDir != filepath.Join(repoRoot, "data") {
		t.Errorf("expected default DataDir %s, got %s", filepath.Join(repoRoot, "data"), cfg.DataDir)
	}
	if cfg.HostAgentURL != "http://127.0.0.1:8081" {
		t.Errorf("expected default HostAgentURL http://127.0.0.1:8081, got %s", cfg.HostAgentURL)
	}
	if cfg.WorkspaceAgentHost != "127.0.0.1" {
		t.Errorf("expected default WorkspaceAgentHost 127.0.0.1, got %s", cfg.WorkspaceAgentHost)
	}
	if cfg.WorkspaceAgentPort != 8765 {
		t.Errorf("expected default WorkspaceAgentPort 8765, got %d", cfg.WorkspaceAgentPort)
	}
	if cfg.WorkspaceImageName != "shadowops-workspace-agent:dev" {
		t.Errorf("expected default WorkspaceImageName shadowops-workspace-agent:dev, got %s", cfg.WorkspaceImageName)
	}
	if cfg.K8sNamespace != "default" {
		t.Errorf("expected default K8sNamespace default, got %s", cfg.K8sNamespace)
	}
	if cfg.PodPoolLimit != 10 {
		t.Errorf("expected default PodPoolLimit 10, got %d", cfg.PodPoolLimit)
	}
	if cfg.PodCPURequest != "200m" {
		t.Errorf("expected default PodCPURequest 200m, got %s", cfg.PodCPURequest)
	}
	if cfg.PodMemoryRequest != "512Mi" {
		t.Errorf("expected default PodMemoryRequest 512Mi, got %s", cfg.PodMemoryRequest)
	}
	if cfg.BrowserHeadless != "false" {
		t.Errorf("expected default BrowserHeadless false, got %s", cfg.BrowserHeadless)
	}
}

func TestLoadConfig_Overrides(t *testing.T) {
	tests := []struct {
		name     string
		env      map[string]string
		validate func(t *testing.T, cfg *Config)
	}{
		{
			name: "custom port and bind addr",
			env: map[string]string{
				"PORT":      "9000",
				"BIND_ADDR": "0.0.0.0:9000",
			},
			validate: func(t *testing.T, cfg *Config) {
				if cfg.Port != "9000" || cfg.BindAddr != "0.0.0.0:9000" {
					t.Errorf("unexpected port/bindAddr: %+v", cfg)
				}
			},
		},
		{
			name: "custom host agent addr without scheme",
			env: map[string]string{
				"HOST_AGENT_ADDR": "host-agent:8081",
			},
			validate: func(t *testing.T, cfg *Config) {
				if cfg.HostAgentURL != "http://host-agent:8081" {
					t.Errorf("expected http://host-agent:8081, got %s", cfg.HostAgentURL)
				}
			},
		},
		{
			name: "custom host agent url with scheme",
			env: map[string]string{
				"HOST_AGENT_URL": "https://host.domain.com:8081/",
			},
			validate: func(t *testing.T, cfg *Config) {
				if cfg.HostAgentURL != "https://host.domain.com:8081" {
					t.Errorf("expected https://host.domain.com:8081, got %s", cfg.HostAgentURL)
				}
			},
		},
		{
			name: "custom k8s namespace and pod limit",
			env: map[string]string{
				"K8S_NAMESPACE":        "prod-workspaces",
				"POD_POOL_LIMIT":       "25",
				"WORKSPACE_AGENT_HOST": "10.0.0.5",
				"WORKSPACE_AGENT_PORT": "9999",
				"POD_CPU_LIMIT":        "1000m",
				"POD_MEMORY_LIMIT":     "2Gi",
				"BROWSER_HEADLESS":     "true",
				"GKE_ENDPOINT":         "https://34.1.2.3",
				"GKE_CA_CERT":          "dGVzdC1jZXJ0",
			},
			validate: func(t *testing.T, cfg *Config) {
				if cfg.K8sNamespace != "prod-workspaces" {
					t.Errorf("expected K8sNamespace prod-workspaces, got %s", cfg.K8sNamespace)
				}
				if cfg.PodPoolLimit != 25 {
					t.Errorf("expected PodPoolLimit 25, got %d", cfg.PodPoolLimit)
				}
				if cfg.WorkspaceAgentHost != "10.0.0.5" {
					t.Errorf("expected WorkspaceAgentHost 10.0.0.5, got %s", cfg.WorkspaceAgentHost)
				}
				if cfg.WorkspaceAgentPort != 9999 {
					t.Errorf("expected WorkspaceAgentPort 9999, got %d", cfg.WorkspaceAgentPort)
				}
				if cfg.PodCPULimit != "1000m" {
					t.Errorf("expected PodCPULimit 1000m, got %s", cfg.PodCPULimit)
				}
				if cfg.PodMemoryLimit != "2Gi" {
					t.Errorf("expected PodMemoryLimit 2Gi, got %s", cfg.PodMemoryLimit)
				}
				if cfg.BrowserHeadless != "true" {
					t.Errorf("expected BrowserHeadless true, got %s", cfg.BrowserHeadless)
				}
				if cfg.GKEEndpoint != "https://34.1.2.3" {
					t.Errorf("expected GKEEndpoint https://34.1.2.3, got %s", cfg.GKEEndpoint)
				}
				if cfg.GKECAData != "dGVzdC1jZXJ0" {
					t.Errorf("expected GKECAData dGVzdC1jZXJ0, got %s", cfg.GKECAData)
				}
			},
		},
		{
			name: "custom gemini model and endpoint",
			env: map[string]string{
				"GEMINI_MODEL": "gemini-1.5-flash",
			},
			validate: func(t *testing.T, cfg *Config) {
				expectedEndpoint := "https://generativelanguage.googleapis.com/v1beta/models/gemini-1.5-flash:generateContent"
				if cfg.GeminiAPIEndpoint != expectedEndpoint {
					t.Errorf("expected %s, got %s", expectedEndpoint, cfg.GeminiAPIEndpoint)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			repoRoot := t.TempDir()
			cfg := LoadConfig(repoRoot)
			tc.validate(t, cfg)
		})
	}
}

func TestGetEnv(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		val      string
		fallback string
		expected string
	}{
		{
			name:     "returns env value when present",
			key:      "TEST_ENV_VAR_EXISTS",
			val:      "custom_value",
			fallback: "default_value",
			expected: "custom_value",
		},
		{
			name:     "returns fallback when env var unset",
			key:      "TEST_ENV_VAR_UNSET_12345",
			val:      "",
			fallback: "default_value",
			expected: "default_value",
		},
		{
			name:     "returns fallback when env var contains only whitespace",
			key:      "TEST_ENV_VAR_WHITESPACE",
			val:      "   ",
			fallback: "default_value",
			expected: "default_value",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.val != "" {
				t.Setenv(tc.key, tc.val)
			}
			got := getEnv(tc.key, tc.fallback)
			if got != tc.expected {
				t.Errorf("getEnv(%q, %q) = %q; want %q", tc.key, tc.fallback, got, tc.expected)
			}
		})
	}
}

func TestServerConfigHelpers(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *Config
		validate func(t *testing.T, s *Server)
	}{
		{
			name: "custom config values",
			cfg: &Config{
				RepoRoot:           "/opt/shadowops",
				HostAgentURL:       "http://custom-host:8081",
				WorkspaceAgentHost: "192.168.1.100",
				WorkspaceAgentPort: 9000,
				DefaultTargetURL:   "https://example.com/login",
				WorkspaceImageName: "custom/agent:v2",
				WorkspacePodImage:  "custom/pod:v1",
				K8sNamespace:       "custom-ns",
				PodCPURequest:      "500m",
				PodMemoryRequest:   "1Gi",
				PodCPULimit:        "2000m",
				PodMemoryLimit:     "4Gi",
				BrowserHeadless:    "true",
				GeminiAPIEndpoint:  "https://custom.gemini.endpoint",
			},
			validate: func(t *testing.T, s *Server) {
				if got := s.hostAgentURL("/containers"); got != "http://custom-host:8081/containers" {
					t.Errorf("hostAgentURL = %q, want http://custom-host:8081/containers", got)
				}
				if got := s.workspaceAgentURL(9001); got != "http://192.168.1.100:9001" {
					t.Errorf("workspaceAgentURL = %q, want http://192.168.1.100:9001", got)
				}
				if got := s.defaultTargetURL(); got != "https://example.com/login" {
					t.Errorf("defaultTargetURL = %q, want https://example.com/login", got)
				}
				if got := s.workspaceImageName(); got != "custom/agent:v2" {
					t.Errorf("workspaceImageName = %q, want custom/agent:v2", got)
				}
				if got := s.geminiEndpoint(); got != "https://custom.gemini.endpoint" {
					t.Errorf("geminiEndpoint = %q, want https://custom.gemini.endpoint", got)
				}
				if got := s.getK8sNamespace(); got != "custom-ns" {
					t.Errorf("getK8sNamespace = %q, want custom-ns", got)
				}
				if got := s.getWorkspacePodImage(); got != "custom/pod:v1" {
					t.Errorf("getWorkspacePodImage = %q, want custom/pod:v1", got)
				}
				if got := s.getWorkspacePodPort(); got != 9000 {
					t.Errorf("getWorkspacePodPort = %d, want 9000", got)
				}
				if got := s.browserHeadless(); got != "true" {
					t.Errorf("browserHeadless = %q, want true", got)
				}
				reqs := s.getPodResourceRequirements()
				if reqs.Requests.Cpu().String() != "500m" {
					t.Errorf("cpu request = %q, want 500m", reqs.Requests.Cpu().String())
				}
				if reqs.Requests.Memory().String() != "1Gi" {
					t.Errorf("memory request = %q, want 1Gi", reqs.Requests.Memory().String())
				}
				if reqs.Limits.Cpu().String() != "2" {
					t.Errorf("cpu limit = %q, want 2", reqs.Limits.Cpu().String())
				}
				if reqs.Limits.Memory().String() != "4Gi" {
					t.Errorf("memory limit = %q, want 4Gi", reqs.Limits.Memory().String())
				}
			},
		},
		{
			name: "default fallbacks when config is empty or invalid resource quantities",
			cfg: &Config{
				PodCPURequest:    "invalid-cpu",
				PodMemoryRequest: "invalid-mem",
				PodCPULimit:      "invalid-lim-cpu",
				PodMemoryLimit:   "invalid-lim-mem",
			},
			validate: func(t *testing.T, s *Server) {
				if got := s.hostAgentURL("/status"); got != "http://127.0.0.1:8081/status" {
					t.Errorf("hostAgentURL = %q, want http://127.0.0.1:8081/status", got)
				}
				if got := s.workspaceAgentURL(8000); got != "http://127.0.0.1:8000" {
					t.Errorf("workspaceAgentURL = %q, want http://127.0.0.1:8000", got)
				}
				if got := s.defaultTargetURL(); got != "https://accounts.google.com/ServiceLogin/signinchooser" {
					t.Errorf("defaultTargetURL = %q, want default Google login URL", got)
				}
				if got := s.workspaceImageName(); got != "shadowops-workspace-agent:dev" {
					t.Errorf("workspaceImageName = %q, want shadowops-workspace-agent:dev", got)
				}
				if got := s.geminiEndpoint(); got != defaultGeminiEndpoint {
					t.Errorf("geminiEndpoint = %q, want %q", got, defaultGeminiEndpoint)
				}
				if got := s.getK8sNamespace(); got != "default" {
					t.Errorf("getK8sNamespace = %q, want default", got)
				}
				if got := s.getWorkspacePodImage(); got != "shadowops-workspace-agent:latest" {
					t.Errorf("getWorkspacePodImage = %q, want shadowops-workspace-agent:latest", got)
				}
				if got := s.getWorkspacePodPort(); got != 8765 {
					t.Errorf("getWorkspacePodPort = %d, want 8765", got)
				}
				if got := s.browserHeadless(); got != "false" {
					t.Errorf("browserHeadless = %q, want false", got)
				}
				// Verify fallback from invalid quantity without panic
				reqs := s.getPodResourceRequirements()
				if reqs.Requests.Cpu().String() != "200m" {
					t.Errorf("cpu fallback = %q, want 200m", reqs.Requests.Cpu().String())
				}
				if reqs.Requests.Memory().String() != "512Mi" {
					t.Errorf("memory fallback = %q, want 512Mi", reqs.Requests.Memory().String())
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := &Server{config: tc.cfg}
			tc.validate(t, s)
		})
	}
}

func TestServeStaticOrSPA(t *testing.T) {
	tempDir := t.TempDir()
	indexContent := "<html><body><h1>ShadowOps Dashboard</h1></body></html>"
	if err := os.WriteFile(filepath.Join(tempDir, "index.html"), []byte(indexContent), 0o644); err != nil {
		t.Fatalf("failed to write index.html: %v", err)
	}
	cssContent := "body { color: red; }"
	if err := os.WriteFile(filepath.Join(tempDir, "styles.css"), []byte(cssContent), 0o644); err != nil {
		t.Fatalf("failed to write styles.css: %v", err)
	}

	cfg := &Config{
		StaticDir: tempDir,
	}
	s := &Server{config: cfg}

	tests := []struct {
		name           string
		path           string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "exact static file",
			path:           "/styles.css",
			expectedStatus: http.StatusOK,
			expectedBody:   cssContent,
		},
		{
			name:           "root path returns index.html",
			path:           "/",
			expectedStatus: http.StatusOK,
			expectedBody:   indexContent,
		},
		{
			name:           "client route falls back to index.html",
			path:           "/workspaces/ws_123",
			expectedStatus: http.StatusOK,
			expectedBody:   indexContent,
		},
		{
			name:           "path traversal attempt rejected with 404",
			path:           "/../../../../etc/passwd",
			expectedStatus: http.StatusNotFound,
			expectedBody:   "404 page not found\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tc.path, nil)
			rec := httptest.NewRecorder()
			s.serveStaticOrSPA(rec, req)

			if rec.Code != tc.expectedStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.expectedStatus)
			}
			if rec.Body.String() != tc.expectedBody {
				t.Errorf("body = %q, want %q", rec.Body.String(), tc.expectedBody)
			}
		})
	}
}
