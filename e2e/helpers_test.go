package e2e_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

// doRequest sends an HTTP request to the test server and returns the status code and parsed body.
func doRequest(t *testing.T, method, path string, body any) (int, map[string]any) {
	t.Helper()

	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, testServerURL+path, reqBody)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request %s %s: %v", method, path, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}

	parsed := map[string]any{}
	if len(raw) > 0 {
		var value any
		if err := json.Unmarshal(raw, &value); err != nil {
			t.Logf("response body (not JSON): %s", string(raw))
		} else {
			switch v := value.(type) {
			case map[string]any:
				parsed = v
			case []any:
				parsed["items"] = v
				parsed["total"] = len(v)
			default:
				parsed["value"] = v
			}
		}
	}

	return resp.StatusCode, parsed
}

// assertStatus fails the test if got != want.
func assertStatus(t *testing.T, got, want int) {
	t.Helper()
	if got != want {
		t.Errorf("status: got %d, want %d", got, want)
	}
}

func parseData(t *testing.T, resp map[string]any) map[string]any {
	t.Helper()
	if data, ok := resp["data"].(map[string]any); ok {
		return data
	}
	if len(resp) == 0 {
		t.Fatalf("response body is empty")
	}
	return resp
}

func parseDataSlice(t *testing.T, resp map[string]any) []any {
	t.Helper()
	if items, ok := resp["items"].([]any); ok {
		return items
	}
	if data, ok := resp["data"].([]any); ok {
		return data
	}
	t.Fatalf("response has no list payload, got: %v", resp)
	return nil
}

// getString extracts a string field from a map.
func getString(t *testing.T, m map[string]any, key string) string {
	t.Helper()
	v, ok := m[key].(string)
	if !ok {
		t.Fatalf("field %q not found or not a string in %v", key, m)
	}
	return v
}
