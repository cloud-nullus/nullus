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

// canonicalCICDTemplateIDs is the seeded CI/CD pipeline template set.
//
// 000053_user_custom_cicd_template 이 web-frontend-v1 을 지우고 web-backend-v1 의
// 이름을 바꿨다. 마이그레이션·메모리 시드·프런트엔드·단위 테스트는 그때 함께
// 갱신됐지만 e2e 만 개수 3 을 그대로 두어 두 달 넘게 실패한 채였다.
var canonicalCICDTemplateIDs = []string{"web-backend-v1", "batch-job-v1"}

// idsOf collects the "id" field of each item in a list payload.
//
// 개수만 세면 시드가 바뀌었을 때 "3개여야 함" 으로만 실패해 무엇이 달라졌는지
// 알 수 없다. 그래서 ID 집합으로 비교한다.
func idsOf(t *testing.T, items []any) []string {
	t.Helper()
	ids := make([]string, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("list item is not an object: %v", item)
		}
		ids = append(ids, getString(t, m, "id"))
	}
	return ids
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
