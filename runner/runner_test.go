package runner_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestGetValueByPath(t *testing.T) {
	tests := []struct {
		name     string
		data     string
		path     string
		expected interface{}
	}{
		{
			name:     "simple field",
			data:     `{"name": "test"}`,
			path:     "name",
			expected: "test",
		},
		{
			name:     "nested field",
			data:     `{"server": {"info": {"name": "server1"}}}`,
			path:     "server.info.name",
			expected: "server1",
		},
		{
			name:     "array index",
			data:     `{"items": ["a", "b", "c"]}`,
			path:     "items[1]",
			expected: "b",
		},
		{
			name:     "array of objects",
			data:     `{"tools": [{"name": "tool1"}, {"name": "tool2"}]}`,
			path:     "tools[0].name",
			expected: "tool1",
		},
		{
			name:     "non-existent path",
			data:     `{"name": "test"}`,
			path:     "notfound",
			expected: nil,
		},
		{
			name:     "empty array access",
			data:     `{"items": []}`,
			path:     "items[0]",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var data map[string]interface{}
			if err := json.Unmarshal([]byte(tt.data), &data); err != nil {
				t.Fatalf("failed to unmarshal JSON: %v", err)
			}

			result := getValueByPath(data, tt.path)

			if tt.expected == nil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
			} else {
				if result != tt.expected {
					t.Errorf("expected %v, got %v", tt.expected, result)
				}
			}
		})
	}
}

func TestCheckExpectations(t *testing.T) {
	tests := []struct {
		name    string
		result  string
		expect  map[string]interface{}
		wantErr bool
		errMsg  string
	}{
		{
			name:   "simple match",
			result: `{"name": "test"}`,
			expect: map[string]interface{}{
				"name": "test",
			},
			wantErr: false,
		},
		{
			name:   "nested match",
			result: `{"server": {"name": "server1"}}`,
			expect: map[string]interface{}{
				"server.name": "server1",
			},
			wantErr: false,
		},
		{
			name:   "mismatch",
			result: `{"name": "test"}`,
			expect: map[string]interface{}{
				"name": "wrong",
			},
			wantErr: true,
			errMsg:  "expected 'wrong', got 'test'",
		},
		{
			name:   "contains match",
			result: `{"text": "Hello World"}`,
			expect: map[string]interface{}{
				"text_contains": "World",
			},
			wantErr: false,
		},
		{
			name:   "contains mismatch",
			result: `{"text": "Hello World"}`,
			expect: map[string]interface{}{
				"text_contains": "NotFound",
			},
			wantErr: true,
			errMsg:  "does not contain",
		},
		{
			name:   "path not found",
			result: `{"name": "test"}`,
			expect: map[string]interface{}{
				"notfound": "value",
			},
			wantErr: true,
			errMsg:  "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var data map[string]interface{}
			if err := json.Unmarshal([]byte(tt.result), &data); err != nil {
				t.Fatalf("failed to unmarshal JSON: %v", err)
			}

			err := checkExpectations(data, tt.expect, false)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestJSONRPCRequest(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		params     map[string]interface{}
		wantMethod string
		wantID     string
	}{
		{
			name:       "initialize request",
			method:     "initialize",
			params:     map[string]interface{}{"protocolVersion": "2025-03-26"},
			wantMethod: "initialize",
		},
		{
			name:       "ping request",
			method:     "ping",
			params:     nil,
			wantMethod: "ping",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := buildJSONRPCRequest(tt.method, tt.params, tt.wantID)

			var parsed map[string]interface{}
			if err := json.Unmarshal([]byte(req), &parsed); err != nil {
				t.Fatalf("failed to parse JSON: %v", err)
			}

			if parsed["jsonrpc"] != "2.0" {
				t.Errorf("expected jsonrpc 2.0, got %v", parsed["jsonrpc"])
			}

			if parsed["method"] != tt.wantMethod {
				t.Errorf("expected method %q, got %v", tt.wantMethod, parsed["method"])
			}
		})
	}
}

// Helper functions that mirror the actual runner implementation

func getValueByPath(data interface{}, path string) interface{} {
	parts := strings.Split(path, ".")
	current := data

	for _, part := range parts {
		if current == nil {
			return nil
		}

		if idx := strings.Index(part, "["); idx != -1 {
			fieldName := part[:idx]
			rest := part[idx:]

			current = getMapValue(current, fieldName)
			if current == nil {
				return nil
			}

			if arr, ok := current.([]interface{}); ok {
				if rest == "[*]" {
					return arr
				}

				rest = rest[1 : len(rest)-1]
				var idx int
				if err := json.Unmarshal([]byte(rest), &idx); err != nil {
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
			current = getMapValue(current, part)
		}
	}

	return current
}

func getMapValue(data interface{}, key string) interface{} {
	switch m := data.(type) {
	case map[string]interface{}:
		return m[key]
	default:
		return nil
	}
}

func checkExpectations(data interface{}, expect map[string]interface{}, stripPrefix bool) error {
	for path, expectedVal := range expect {
		if path == "http_status" || path == "error" {
			continue
		}

		contains := false
		actualPath := path
		if strings.HasSuffix(path, "_contains") {
			contains = true
			actualPath = strings.TrimSuffix(path, "_contains")
		}

		if stripPrefix && strings.HasPrefix(actualPath, "result.") {
			actualPath = strings.TrimPrefix(actualPath, "result.")
		}

		actualVal := getValueByPath(data, actualPath)
		if actualVal == nil {
			return &expectError{msg: "path '" + actualPath + "' not found"}
		}

		if contains {
			if !strings.Contains(fmt.Sprintf("%v", actualVal), fmt.Sprintf("%v", expectedVal)) {
				return &expectError{msg: "does not contain"}
			}
		} else {
			expectedStr := fmt.Sprintf("%v", expectedVal)
			actualStr := fmt.Sprintf("%v", actualVal)
			if expectedStr != actualStr {
				return &expectError{msg: "expected '" + expectedStr + "', got '" + actualStr + "'"}
			}
		}
	}
	return nil
}

type expectError struct {
	msg string
}

func (e *expectError) Error() string {
	return e.msg
}

func buildJSONRPCRequest(method string, params map[string]interface{}, id string) string {
	if params == nil {
		params = make(map[string]interface{})
	}

	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	}

	data, _ := json.Marshal(req)
	return string(data)
}
