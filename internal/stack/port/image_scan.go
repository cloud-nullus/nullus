package port

import (
	"context"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// InstalledImageScanner 는 스택 네임스페이스에서 돌고 있는 이미지를 찾아 스택의
// 스캐너로 스캔한다. 이미지마다 결과를 돌려주고, 스캔하지 못한 이미지도 빠뜨리지
// 않는다(상태 failed).
type InstalledImageScanner interface {
	ScanInstalledImages(ctx context.Context, kubeconfig []byte, namespace string) ([]domain.StackImageScan, error)
}

// StackImageScanRepository 는 스택의 설치 이미지 스캔 결과를 보관한다.
type StackImageScanRepository interface {
	// ReplaceForStack 은 스택의 결과를 이번 스캔으로 통째로 바꾼다 — 더는 돌지 않는
	// 이미지의 결과가 남아 현재 상태처럼 보이지 않게 한다.
	ReplaceForStack(ctx context.Context, stackID string, scans []domain.StackImageScan) error
	ListByStack(ctx context.Context, stackID string) ([]domain.StackImageScan, error)
	// ListVulnerabilities 는 이미지 하나의 저장된 취약점 목록이다.
	ListVulnerabilities(ctx context.Context, stackID, digest string) (domain.StackImageVulnerabilities, error)
}

// CompletedStackLister 는 설치가 끝난 스택을 조직과 무관하게 돌려준다.
// 설치 이미지 주기 재스캔이 쓴다 — 그것은 조직 경계와 무관한 일이다.
type CompletedStackLister interface {
	ListCompleted(ctx context.Context) ([]*domain.Stack, error)
}
