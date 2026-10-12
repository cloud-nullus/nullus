package handler_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 시크릿 탐지 동작을 경고·끄기로 바꿀 수 있다. 기본은 차단이고, 이 필드를 모르는 옛 요청은 저장된 값을 그대로 둔다.
func TestScanPolicyHandler_SecretDetection(t *testing.T) {
	e, _ := newScanPolicyEcho(t)

	_, before := doPolicyRequest(t, e, http.MethodGet, "")
	assert.Equal(t, "block", before.Policy.SecretDetection)

	rec, resp := doPolicyRequest(t, e, http.MethodPut,
		`{"block_severity":"CRITICAL","ignore_unfixed":true,"on_scanner_unreachable":"block","secret_detection":"warn"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "warn", resp.Policy.SecretDetection)

	rec, resp = doPolicyRequest(t, e, http.MethodPut,
		`{"block_severity":"HIGH","ignore_unfixed":true,"on_scanner_unreachable":"block"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "warn", resp.Policy.SecretDetection, "옛 클라이언트의 저장이 운영자가 바꾼 값을 되돌리면 안 된다")

	rec, resp = doPolicyRequest(t, e, http.MethodPut,
		`{"block_severity":"HIGH","ignore_unfixed":true,"on_scanner_unreachable":"block","secret_detection":"OFF"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "off", resp.Policy.SecretDetection, "대소문자는 받아 준다")
}

func TestScanPolicyHandler_RejectsInvalidSecretDetection(t *testing.T) {
	e, _ := newScanPolicyEcho(t)
	rec, _ := doPolicyRequest(t, e, http.MethodPut,
		`{"block_severity":"CRITICAL","ignore_unfixed":true,"on_scanner_unreachable":"block","secret_detection":"disable"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "INVALID_SCAN_POLICY")
}
