package prometheus

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

type VectorResult struct {
	Metric map[string]string
	Value  float64
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (c *Client) Query(ctx context.Context, promql string) (float64, error) {
	results, err := c.QueryVector(ctx, promql)
	if err != nil {
		return 0, err
	}
	if len(results) == 0 {
		return 0, fmt.Errorf("prometheus query returned empty result")
	}
	return results[0].Value, nil
}

func (c *Client) QueryVector(ctx context.Context, promql string) ([]VectorResult, error) {
	u, err := url.Parse(c.baseURL + "/api/v1/query")
	if err != nil {
		return nil, fmt.Errorf("parse prometheus query url: %w", err)
	}

	q := u.Query()
	q.Set("query", promql)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create prometheus query request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute prometheus query request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024)) // #nosec G104 -- best-effort body read for error context
		return nil, fmt.Errorf("prometheus query failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload prometheusQueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode prometheus query response: %w", err)
	}

	if payload.Status != "success" {
		if payload.Error != "" {
			return nil, fmt.Errorf("prometheus query error: %s", payload.Error)
		}
		return nil, fmt.Errorf("prometheus query status: %s", payload.Status)
	}

	results := make([]VectorResult, 0, len(payload.Data.Result))
	for _, item := range payload.Data.Result {
		if len(item.Value) < 2 {
			return nil, fmt.Errorf("invalid prometheus vector value")
		}
		valueStr, ok := item.Value[1].(string)
		if !ok {
			return nil, fmt.Errorf("invalid prometheus value type")
		}
		value, err := strconv.ParseFloat(valueStr, 64)
		if err != nil {
			return nil, fmt.Errorf("parse prometheus value %q: %w", valueStr, err)
		}

		results = append(results, VectorResult{
			Metric: item.Metric,
			Value:  value,
		})
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("prometheus query returned empty result")
	}

	return results, nil
}

type prometheusQueryResponse struct {
	Status string `json:"status"`
	Error  string `json:"error"`
	Data   struct {
		ResultType string                     `json:"resultType"`
		Result     []prometheusVectorResponse `json:"result"`
	} `json:"data"`
}

type prometheusVectorResponse struct {
	Metric map[string]string `json:"metric"`
	Value  []any             `json:"value"`
}
