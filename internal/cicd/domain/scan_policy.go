package domain

import (
	"errors"
	"fmt"
	"strings"
)

// Severity 는 Trivy 가 매기는 취약점 등급이다.
type Severity string

const (
	SeverityCritical Severity = "CRITICAL"
	SeverityHigh     Severity = "HIGH"
	SeverityMedium   Severity = "MEDIUM"
	SeverityLow      Severity = "LOW"
)

// severityOrder 는 높은 등급부터다. UNKNOWN 은 넣지 않는다 — 등급을 모르는 것을
// 차단 기준으로 삼으면 기준이 무엇인지 설명할 수 없다.
var severityOrder = []Severity{SeverityCritical, SeverityHigh, SeverityMedium, SeverityLow}

// UnreachableAction 은 스캔을 수행하지 못했을 때(스캐너 도달 불가 등)의 동작이다.
type UnreachableAction string

const (
	// UnreachableBlock 은 멈춘다. 기본값이다 — 스캐너가 죽은 동안 모든 배포가
	// 초록불로 지나가는 것이 가장 나쁜 실패다 (설계 §6.1).
	UnreachableBlock UnreachableAction = "block"
	// UnreachableAllow 는 통과시킨다. 긴급 배포용 우회로다 — 우회로가 없으면
	// 사람들은 파이프라인 자체를 우회한다.
	UnreachableAllow UnreachableAction = "allow"
)

// ErrInvalidScanPolicy 는 정책 값이 허용 범위를 벗어났을 때의 오류다.
var ErrInvalidScanPolicy = errors.New("스캔 정책이 올바르지 않습니다")

// Validate 는 저장·푸시 전에 정책 값을 확인한다.
//
// 모르는 값을 기본값으로 바꿔 받지 않는다. 오타(HIGHT)가 조용히 CRITICAL 로
// 바뀌면 운영자는 HIGH 를 막는 줄 알고 있게 된다.
func (p ScanPolicy) Validate() error {
	if severityRank(p.BlockSeverity) < 0 {
		return fmt.Errorf("%w: 차단 심각도 %q 는 CRITICAL·HIGH·MEDIUM·LOW 중 하나여야 합니다",
			ErrInvalidScanPolicy, p.BlockSeverity)
	}
	switch p.OnScannerUnreachable {
	case UnreachableBlock, UnreachableAllow:
	default:
		return fmt.Errorf("%w: 스캐너 장애 시 동작 %q 는 block·allow 중 하나여야 합니다",
			ErrInvalidScanPolicy, p.OnScannerUnreachable)
	}
	return nil
}

// TrivySeverities 는 게이트 명령의 --severity 값이다.
//
// Trivy 의 --severity 는 "이 등급만" 이다. "이 등급 이상" 을 뜻하려면 위 등급을
// 모두 적어야 한다 — HIGH 만 적으면 CRITICAL 이 통과한다.
func (p ScanPolicy) TrivySeverities() string {
	block := p.blockRank()
	parts := make([]string, 0, block+1)
	for i := block; i >= 0; i-- {
		parts = append(parts, string(severityOrder[i]))
	}
	return strings.Join(parts, ",")
}

// UnreachableActionOrDefault 는 비어 있으면 기본값(block)이다.
func (p ScanPolicy) UnreachableActionOrDefault() UnreachableAction {
	if p.OnScannerUnreachable == "" {
		return UnreachableBlock
	}
	return p.OnScannerUnreachable
}

// blockRank 는 차단 등급의 순위다. 0 값 정책은 기본 기준(CRITICAL)으로 읽는다 —
// 정책을 명시하지 않은 호출이 아무것도 막지 않게 되면 안 된다.
func (p ScanPolicy) blockRank() int {
	if r := severityRank(p.BlockSeverity); r >= 0 {
		return r
	}
	return severityRank(SeverityCritical)
}

func severityRank(s Severity) int {
	for i, v := range severityOrder {
		if v == s {
			return i
		}
	}
	return -1
}

func countAt(c SeverityCounts, s Severity) int {
	switch s {
	case SeverityCritical:
		return c.Critical
	case SeverityHigh:
		return c.High
	case SeverityMedium:
		return c.Medium
	case SeverityLow:
		return c.Low
	}
	return 0
}
