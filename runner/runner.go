package runner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/treere/mcp-test-suite/config"
)

type Runner struct {
	config     *config.Config
	httpClient *http.Client
	sessionID  string
	results    Results
}

type Results struct {
	Tests  []TestResult `json:"tests"`
	Passed int          `json:"passed"`
	Failed int          `json:"failed"`
	Total  int          `json:"total"`
}

type TestResult struct {
	Name       string      `json:"name"`
	Passed     bool        `json:"passed"`
	Error      interface{} `json:"error,omitempty"`
	HTTPStatus int         `json:"http_status,omitempty"`
}

func New(cfg *config.Config) *Runner {
	return &Runner{
		config: cfg,
		httpClient: &http.Client{
			Timeout: cfg.Server.Timeout,
		},
	}
}

func (r *Runner) Run() *Results {
	r.results = Results{
		Tests: make([]TestResult, len(r.config.Tests)),
	}

	fmt.Println("==========================================")
	fmt.Println("MCP Integration Test Suite")
	fmt.Println("==========================================")
	fmt.Printf("Server: %s\n", r.config.Server.URL)
	fmt.Printf("Tests: %d\n", len(r.config.Tests))
	fmt.Println("==========================================")

	for i, tc := range r.config.Tests {
		r.runTest(tc, &r.results.Tests[i])
	}

	r.results.Total = len(r.results.Tests)
	for _, t := range r.results.Tests {
		if t.Passed {
			r.results.Passed++
		} else {
			r.results.Failed++
		}
	}

	return &r.results
}

func (r *Runner) runTest(tc config.TestCase, result *TestResult) {
	fmt.Printf("\n=== Testing: %s ===\n", tc.Name)

	// Build the JSON-RPC request
	id := fmt.Sprintf("req_%d", result.HTTPStatus) // dummy id
	rpcRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  tc.Method,
	}

	// Ensure params is never null
	params := tc.Params
	if params == nil {
		params = make(map[string]interface{})
	}
	rpcRequest["params"] = params

	body, err := json.Marshal(rpcRequest)
	if err != nil {
		result.Passed = false
		result.Error = fmt.Sprintf("failed to marshal request: %v", err)
		return
	}

	req, err := http.NewRequest("POST", r.config.Server.URL, bytes.NewReader(body))
	if err != nil {
		result.Passed = false
		result.Error = fmt.Sprintf("failed to create request: %v", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	if tc.Session && r.sessionID != "" {
		req.Header.Set("mcp-session-id", r.sessionID)
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		result.Passed = false
		result.Error = fmt.Sprintf("request failed: %v", err)
		return
	}
	defer resp.Body.Close()

	result.HTTPStatus = resp.StatusCode

	// Extract session ID from response headers
	if tc.Session && r.sessionID == "" {
		if sessionID := resp.Header.Get("mcp-session-id"); sessionID != "" {
			r.sessionID = sessionID
		}
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		result.Passed = false
		result.Error = fmt.Sprintf("failed to read response: %v", err)
		return
	}

	fmt.Printf("Response (HTTP %d)\n", resp.StatusCode)

	// Check HTTP status expectation
	if expectedStatus, ok := tc.Expect["http_status"]; ok {
		if status := int(expectedStatus.(int)); status != resp.StatusCode {
			result.Passed = false
			result.Error = fmt.Sprintf("expected HTTP %d, got %d", status, resp.StatusCode)
			return
		}
		// If only checking HTTP status and it's a notification (no body), we're done
		if !strings.Contains(tc.Method, "/") || strings.HasPrefix(tc.Method, "notifications/") {
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				result.Passed = true
				return
			}
		}
	}

	// Check for error expectation
	if _, hasError := tc.Expect["error"]; hasError {
		var respJSON map[string]interface{}
		if err := json.Unmarshal(respBody, &respJSON); err == nil {
			if _, hasErr := respJSON["error"]; hasErr {
				result.Passed = true
				return
			}
		}
		result.Passed = false
		result.Error = "expected error response, got success"
		return
	}

	// Parse response and check expectations
	var respJSON map[string]interface{}
	if err := json.Unmarshal(respBody, &respJSON); err != nil {
		result.Passed = false
		result.Error = fmt.Sprintf("failed to parse response: %v", err)
		return
	}

	// Check for result
	resultVal, hasResult := respJSON["result"]
	if !hasResult {
		result.Passed = false
		result.Error = fmt.Sprintf("no result in response: %s", string(respBody))
		return
	}

	// Check expectations (strip "result." prefix since we're already checking inside result)
	if err := r.checkExpectations(resultVal, tc.Expect, true); err != nil {
		result.Passed = false
		result.Error = err.Error()
		return
	}

	result.Passed = true
}

func (r *Runner) checkExpectations(result interface{}, expect map[string]interface{}, stripResultPrefix bool) error {
	for path, expectedVal := range expect {
		if path == "http_status" || path == "error" {
			continue
		}

		// Handle _contains suffix for partial string matching
		contains := false
		actualPath := path
		if strings.HasSuffix(path, "_contains") {
			contains = true
			actualPath = strings.TrimSuffix(path, "_contains")
		}

		// Strip "result." prefix if present and we're checking inside result
		if stripResultPrefix && strings.HasPrefix(actualPath, "result.") {
			actualPath = strings.TrimPrefix(actualPath, "result.")
		}

		actualVal := r.getValueByPath(result, actualPath)
		if actualVal == nil {
			return fmt.Errorf("path '%s' not found in result", actualPath)
		}

		if contains {
			if !strings.Contains(fmt.Sprintf("%v", actualVal), fmt.Sprintf("%v", expectedVal)) {
				return fmt.Errorf("path '%s' does not contain '%v', got '%v'", actualPath, expectedVal, actualVal)
			}
		} else {
			// Handle array wildcard notation result.tools[*].name
			if strings.Contains(actualPath, "[*]") {
				if !r.checkArrayContains(result, actualPath, expectedVal) {
					return fmt.Errorf("array at '%s' does not contain '%v'", actualPath, expectedVal)
				}
			} else {
				expectedStr := fmt.Sprintf("%v", expectedVal)
				actualStr := fmt.Sprintf("%v", actualVal)
				if expectedStr != actualStr {
					return fmt.Errorf("path '%s': expected '%s', got '%s'", path, expectedStr, actualStr)
				}
			}
		}
	}
	return nil
}

func (r *Runner) getValueByPath(data interface{}, path string) interface{} {
	parts := strings.Split(path, ".")
	current := data

	for _, part := range parts {
		if current == nil {
			return nil
		}

		// Handle array index notation: tools[0].name
		if idx := strings.Index(part, "["); idx != -1 {
			fieldName := part[:idx]
			rest := part[idx:]

			current = r.getMapValue(current, fieldName)
			if current == nil {
				return nil
			}

			// Parse array index
			if arr, ok := current.([]interface{}); ok {
				// Handle [*] wildcard
				if rest == "[*]" {
					return arr
				}

				rest = rest[1 : len(rest)-1] // Remove brackets
				var idx int
				if _, err := fmt.Sscanf(rest, "%d", &idx); err != nil {
					return nil
				}

				if idx < 0 || idx >= len(arr) {
					return nil
				}
				current = arr[idx]
			} else {
				return nil
			}
		} else {
			current = r.getMapValue(current, part)
		}
	}

	return current
}

func (r *Runner) getMapValue(data interface{}, key string) interface{} {
	switch m := data.(type) {
	case map[string]interface{}:
		return m[key]
	default:
		return nil
	}
}

func (r *Runner) checkArrayContains(data interface{}, path string, expected interface{}) bool {
	parts := strings.Split(path, ".")
	arrPath := parts[len(parts)-1]

	// Get the array
	arr := r.getValueByPath(data, arrPath)
	if arr == nil {
		return false
	}

	slice, ok := arr.([]interface{})
	if !ok {
		return false
	}

	expectedStr := fmt.Sprintf("%v", expected)

	for _, item := range slice {
		// Check if item matches expected value directly
		if fmt.Sprintf("%v", item) == expectedStr {
			return true
		}

		// Check if item is a map and has a field matching expected
		if m, ok := item.(map[string]interface{}); ok {
			// The path might have additional field specification
			fieldName := strings.TrimPrefix(parts[len(parts)-1], "[*]")
			if fieldName != "[*]" && fieldName != "" {
				if val, ok := m[fieldName]; ok {
					if fmt.Sprintf("%v", val) == expectedStr {
						return true
					}
				}
			}
		}
	}

	return false
}
