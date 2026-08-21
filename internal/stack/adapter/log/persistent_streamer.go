package log

import (
	"context"
	"log/slog"
	"time"

	"github.com/cloud-nullus/draft/internal/stack/port"
)

// storeTimeout 은 저장소 한 번 호출의 상한이다.
//
// 로그 기록은 설치의 곁가지다. DB 가 흔들릴 때 설치 고루틴이 거기 매달리면
// 곁가지가 본체를 멈춘다.
const storeTimeout = 3 * time.Second

// DeployLogStore 는 설치 로그를 프로세스 밖에 남긴다.
type DeployLogStore interface {
	Append(ctx context.Context, deploymentID string, entry port.LogEntry) error
	List(ctx context.Context, deploymentID string, limit int) ([]port.LogEntry, error)
	Delete(ctx context.Context, deploymentID string) error
}

// PersistentStreamer 는 메모리 스트리머에 저장소를 덧댄다.
//
// 실시간 팬아웃은 그대로 메모리가 맡고, 같은 항목을 저장소에도 남긴다.
// 파드가 재시작되면 메모리 이력은 사라지지만 저장소에는 남아 있어, 재접속한
// 화면이 그동안의 로그를 되돌려 받는다 — 설치는 20~30분짜리라 그 사이 재시작이
// 겹치면 무엇이 왜 멈췄는지 알 방법이 없었다.
type PersistentStreamer struct {
	memory *MemoryStreamer
	store  DeployLogStore
}

// NewPersistentStreamer 는 저장소를 덧댄 스트리머를 만든다.
// store 가 nil 이면 메모리 스트리머와 똑같이 동작한다.
func NewPersistentStreamer(memory *MemoryStreamer, store DeployLogStore) *PersistentStreamer {
	if memory == nil {
		memory = NewMemoryStreamer()
	}
	return &PersistentStreamer{memory: memory, store: store}
}

// Stream 은 항목을 저장소에 남기고 구독자에게 보낸다.
//
// 저장 실패는 스트리밍을 막지 않는다. 로그를 남기지 못하는 것이 설치를 멈출
// 이유는 아니다 — 실패 사실만 서버 로그에 남긴다.
func (s *PersistentStreamer) Stream(ctx context.Context, deploymentID string, entry port.LogEntry) {
	if s.store != nil {
		storeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), storeTimeout)
		if err := s.store.Append(storeCtx, deploymentID, entry); err != nil {
			slog.Warn("deploy log persist failed", "deployment_id", deploymentID, "error", err)
		}
		cancel()
	}
	s.memory.Stream(ctx, deploymentID, entry)
}

// Subscribe 는 그동안의 로그를 돌려주고 이후 항목을 잇는다.
//
// 이 프로세스가 그 배포를 스트리밍했으면 메모리가 진실이다. 저장소를 겹쳐 읽으면
// 같은 줄이 두 번 보인다. 메모리가 비어 있을 때만 — 즉 재시작 뒤에만 —
// 저장소에서 읽는다.
func (s *PersistentStreamer) Subscribe(deploymentID string) <-chan port.LogEntry {
	if s.store == nil || s.memory.HasHistory(deploymentID) {
		return s.memory.Subscribe(deploymentID)
	}

	ctx, cancel := context.WithTimeout(context.Background(), storeTimeout)
	defer cancel()

	stored, err := s.store.List(ctx, deploymentID, maxHistoryEntries)
	if err != nil {
		slog.Warn("deploy log replay failed", "deployment_id", deploymentID, "error", err)
		return s.memory.Subscribe(deploymentID)
	}
	return s.memory.SubscribeWithHistory(deploymentID, stored)
}

func (s *PersistentStreamer) Unsubscribe(deploymentID string, ch <-chan port.LogEntry) {
	s.memory.Unsubscribe(deploymentID, ch)
}

// ClearHistory 는 이 배포의 로그를 메모리와 저장소 양쪽에서 지운다.
//
// 한쪽만 지우면 새 실행이 이전 실행의 로그 위에 겹쳐 쌓인다.
func (s *PersistentStreamer) ClearHistory(deploymentID string) {
	s.memory.ClearHistory(deploymentID)
	if s.store == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), storeTimeout)
	defer cancel()
	if err := s.store.Delete(ctx, deploymentID); err != nil {
		slog.Warn("deploy log clear failed", "deployment_id", deploymentID, "error", err)
	}
}
