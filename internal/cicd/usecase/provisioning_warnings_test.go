package usecase

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// 프로비저닝 경고는 응답으로만 돌아갔다. 화면이 그것을 버리면(#208 이전) 흔적이
// 통째로 사라져, 저장소는 만들어졌는데 Argo CD Application 이 왜 없는지 사후에
// 알 방법이 없었다 — 2026-08-21 운영에서 그랬다.
//
// 클라이언트가 읽든 말든 서버 로그에는 남아야 한다.
func TestLogProvisioningWarnings_LeavesATraceOnTheServer(t *testing.T) {
	var buf bytes.Buffer
	restore := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	defer slog.SetDefault(restore)

	logProvisioningWarnings("orders-api", "stk-1", &ProvisionPipelineRepositoryOutput{
		Warnings:         []string{"Argo CD Application 적용 실패: connection refused"},
		MissingVariables: []string{"HARBOR_PASSWORD"},
	})

	out := buf.String()
	assert.Contains(t, out, "orders-api")
	assert.Contains(t, out, "stk-1")
	assert.Contains(t, out, "Argo CD Application")
	assert.Contains(t, out, "HARBOR_PASSWORD")
}

// 문제가 없으면 조용하다. 매번 찍으면 진짜 경고가 묻힌다.
func TestLogProvisioningWarnings_SilentWhenClean(t *testing.T) {
	var buf bytes.Buffer
	restore := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	defer slog.SetDefault(restore)

	logProvisioningWarnings("orders-api", "stk-1", &ProvisionPipelineRepositoryOutput{})

	assert.Equal(t, "", strings.TrimSpace(buf.String()))
}
