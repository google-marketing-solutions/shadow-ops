package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"golang.org/x/oauth2/google"
)

const defaultGeminiEndpoint = "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-pro:generateContent"

func (s *Server) geminiEndpoint() string {
	if s != nil && s.config != nil && s.config.GeminiAPIEndpoint != "" {
		return s.config.GeminiAPIEndpoint
	}
	return defaultGeminiEndpoint
}

type GeminiRequest struct {
	Contents         []Content `json:"contents"`
	GenerationConfig `json:"generationConfig"`
}

type Content struct {
	Parts []Part `json:"parts"`
}

type Part struct {
	Text string `json:"text"`
}

type GenerationConfig struct {
	Temperature     float32  `json:"temperature"`
	TopK            int      `json:"topK"`
	TopP            float32  `json:"topP"`
	MaxOutputTokens int      `json:"maxOutputTokens"`
	StopSequences   []string `json:"stopSequences"`
}
type GeminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error,omitempty"`
}

type GeminiFeedback struct {
	Error     string `json:"error,omitempty"`
	Traceback string `json:"traceback,omitempty"`
	Logs      string `json:"logs,omitempty"`
}

func (s *Server) generatePlaywrightScript(actionSet *ActionSet, feedback *GeminiFeedback) (string, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")

	playwrightCodeMD, err := os.ReadFile("../PLAYWRIGHT_CODE.md")
	if err != nil {
		return "", fmt.Errorf("failed to read PLAYWRIGHT_CODE.md: %w", err)
	}

	actionSetJSON, err := json.MarshalIndent(actionSet, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal action set to JSON: %w", err)
	}

	feedbackContext := ""
	if feedback != nil && (feedback.Error != "" || feedback.Logs != "") {
		feedbackContext = fmt.Sprintf(`
IMPORTANT: A previous version of the script failed. Use the following execution context to fix the issues.
ERROR: %s
TRACEBACK: %s
EXECUTION LOGS:
%s
`, feedback.Error, feedback.Traceback, feedback.Logs)
	}

	prompt := fmt.Sprintf(`
You are an expert Playwright automation engineer. Your task is to write a Python script that automates a web workflow based on a provided Action Set.

Follow these instructions carefully:
%s

%s

Here is the Action Set data in JSON format. Use it as a reference to understand the user's workflow and to create stable selectors.

Action Set:
%s

Based on these instructions and the Action Set, generate a complete Python Playwright script. The script should be robust, readable, and include the required 'run' function.
	`, string(playwrightCodeMD), feedbackContext, string(actionSetJSON))

	reqBody := GeminiRequest{
		Contents: []Content{
			{
				Parts: []Part{
					{Text: prompt},
				},
			},
		},
		GenerationConfig: GenerationConfig{
			Temperature:     0.2,
			TopK:            1,
			TopP:            0.95,
			MaxOutputTokens: 8192,
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request body: %w", err)
	}

	endpoint := s.geminiEndpoint()
	if apiKey != "" {
		if strings.Contains(endpoint, "?") {
			endpoint = fmt.Sprintf("%s&key=%s", endpoint, apiKey)
		} else {
			endpoint = fmt.Sprintf("%s?key=%s", endpoint, apiKey)
		}
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	if apiKey == "" {
		// Use Application Default Credentials
		ctx := context.Background()
		creds, err := google.FindDefaultCredentials(ctx, "https://www.googleapis.com/auth/cloud-platform")
		if err != nil {
			return "", fmt.Errorf("failed to find default credentials: %w", err)
		}
		token, err := creds.TokenSource.Token()
		if err != nil {
			return "", fmt.Errorf("failed to get token: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token.AccessToken)

		projectID, err := getProjectID()
		if err != nil {
			return "", err
		}
		req.Header.Set("x-goog-user-project", projectID)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request to Gemini API: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Gemini API request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var geminiResp GeminiResponse
	if err := json.Unmarshal(respBody, &geminiResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if geminiResp.Error.Message != "" {
		return "", fmt.Errorf("gemini API error: %s", geminiResp.Error.Message)
	}
	if len(geminiResp.Candidates) > 0 && len(geminiResp.Candidates[0].Content.Parts) > 0 {
		rawScript := geminiResp.Candidates[0].Content.Parts[0].Text
		// Regex to find python code inside markdown block
		re := regexp.MustCompile("(?s)```python\n(.*?)\n```")
		matches := re.FindStringSubmatch(rawScript)
		if len(matches) > 1 {
			return strings.TrimSpace(matches[1]), nil // Return the captured group
		}
		// Fallback if no markdown block is found, return the raw text
		return strings.TrimSpace(rawScript), nil
	}

	return "", fmt.Errorf("no script was generated")
}

func getProjectID() (string, error) {
	ctx := context.Background()
	creds, err := google.FindDefaultCredentials(ctx, "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		return "", fmt.Errorf("failed to find default credentials: %w", err)
	}
	if creds.ProjectID == "" {
		return "", fmt.Errorf("google default credentials did not provide a project ID")
	}
	return creds.ProjectID, nil
}
