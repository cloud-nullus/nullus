package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

type stubStackReader struct {
	summary *port.StackSummary
	err     error
}

func (s *stubStackReader) GetStackSummary(context.Context, string) (*port.StackSummary, error) {
	return s.summary, s.err
}

func TestOTLPEndpointFor_ReturnsCollectorAddress(t *testing.T) {
	reader := &stubStackReader{summary: &port.StackSummary{
		OTLPEndpoint: "otel-collector-opentelemetry-collector.nullus-otelstack.svc.cluster.local:4317",
	}}

	got := OTLPEndpointFor(context.Background(), reader, "stk_1")
	assert.Equal(t, "otel-collector-opentelemetry-collector.nullus-otelstack.svc.cluster.local:4317", got)
}

// 관측은 부가 기능이다. 조회가 실패했다고 배포가 막히면 안 되고, 닿지 않는
// 주소를 넣어도 안 된다 — 앱이 영원히 재시도하며 오류 로그만 쌓는다.
func TestOTLPEndpointFor_EmptyWhenUnavailable(t *testing.T) {
	cases := map[string]struct {
		reader  port.StackReader
		stackID string
	}{
		"배선 안 됨":   {reader: nil, stackID: "stk_1"},
		"스택 ID 없음": {reader: &stubStackReader{}, stackID: ""},
		"조회 실패":    {reader: &stubStackReader{err: errors.New("db down")}, stackID: "stk_1"},
		"스택 없음":    {reader: &stubStackReader{summary: nil}, stackID: "stk_1"},
		"수집기 미설치":  {reader: &stubStackReader{summary: &port.StackSummary{}}, stackID: "stk_1"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Empty(t, OTLPEndpointFor(context.Background(), tc.reader, tc.stackID))
		})
	}
}
