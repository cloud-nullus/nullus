package prometheus

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_Query_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/query", r.URL.Path)
		require.Equal(t, "test_query", r.URL.Query().Get("query"))

		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"status": "success",
			"data": map[string]any{
				"resultType": "vector",
				"result": []any{
					map[string]any{"metric": map[string]string{}, "value": []any{1234567890, "42.5"}},
				},
			},
		}))
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	val, err := client.Query(context.Background(), "test_query")
	require.NoError(t, err)
	assert.InDelta(t, 42.5, val, 0.01)
}

func TestClient_Query_HTTPFailure(t *testing.T) {
	client := NewClient("http://127.0.0.1:1")

	_, err := client.Query(context.Background(), "test_query")
	require.Error(t, err)
}

func TestClient_Query_EmptyResult(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"status": "success",
			"data": map[string]any{
				"resultType": "vector",
				"result":     []any{},
			},
		}))
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	_, err := client.Query(context.Background(), "test_query")
	require.Error(t, err)
}
