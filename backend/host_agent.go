package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// hostAgentURL constructs the full target URL for a host-agent request, respecting Config and env vars.
func (s *Server) hostAgentURL(path string) string {
	baseURL := "http://127.0.0.1:8081"
	if s != nil && s.config != nil && s.config.HostAgentURL != "" {
		baseURL = s.config.HostAgentURL
	}

	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return baseURL + path
}

func (s *Server) callHostAgent(ctx context.Context, method, path string, body any) ([]byte, int, error) {
	url := s.hostAgentURL(path)

	var reqBody io.Reader
	if body != nil {
		jsonBytes, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to marshal body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := s.httpClient
	if client == nil {
		client = &http.Client{}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to call host agent at %s: %w", url, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read response body: %w", err)
	}

	return respBody, resp.StatusCode, nil
}
