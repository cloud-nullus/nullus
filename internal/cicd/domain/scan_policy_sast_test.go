package domain

import (
	"errors"
	"testing"
)

// SAST 의 기본도 차단이다 — Trivy 이미지 스캔과 같다(사용자 결정).
func TestDefaultScanPolicy_BlocksOnSASTGateFailure(t *testing.T) {
	if got := DefaultScanPolicy().SASTOnGateFailure; got != SASTGateBlock {
		t.Fatalf("기본은 Quality Gate 실패 시 차단이어야 한다, got %q", got)
	}
}

// 이 필드를 모르는 옛 클라이언트·저장 행은 빈 값이다. 빈 값을 경고로 읽으면 조용히
// 게이트가 풀린다.
func TestScanPolicy_EmptySASTActionMeansBlock(t *testing.T) {
	if got := (ScanPolicy{}).SASTGateActionOrDefault(); got != SASTGateBlock {
		t.Fatalf("빈 값은 차단이어야 한다, got %q", got)
	}
}

func TestScanPolicy_ValidateSASTAction(t *testing.T) {
	base := DefaultScanPolicy()
	for _, ok := range []SASTGateAction{SASTGateBlock, SASTGateWarn, ""} {
		p := base
		p.SASTOnGateFailure = ok
		if err := p.Validate(); err != nil {
			t.Fatalf("%q 는 허용해야 한다: %v", ok, err)
		}
	}
	p := base
	p.SASTOnGateFailure = "warning"
	if err := p.Validate(); !errors.Is(err, ErrInvalidScanPolicy) {
		t.Fatalf("오타를 기본값으로 바꿔 받으면 운영자는 경고인 줄 알고 차단 중이게 된다: %v", err)
	}
}
