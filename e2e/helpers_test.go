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

	var parsed map[string]any
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &parsed); err != nil {
			t.Logf("response body (not JSON): %s", string(raw))
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

// parseData extracts the "data" field from a response map.
func parseData(t *testing.T, resp map[string]any) map[string]any {
	t.Helper()
	data, ok := resp["data"].(map[string]any)
	if !ok {
		t.Fatalf("response missing 'data' field, got: %v", resp)
	}
	return data
}

// parseDataSlice extracts "data" as a slice from a response map.
func parseDataSlice(t *testing.T, resp map[string]any) []any {
	t.Helper()
	data, ok := resp["data"].([]any)
	if !ok {
		t.Fatalf("response 'data' is not a slice, got: %v", resp)
	}
	return data
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
