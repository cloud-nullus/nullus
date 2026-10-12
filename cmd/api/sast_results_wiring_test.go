package main

import (
	"os"
	"strings"
	"testing"
)

// SAST 결과는 두 곳에 배선돼야 한다 — 실행 기록 동기화가 남기고, 파이프라인 핸들러가 내준다.
// 동기화 쪽이 빠지면 화면은 503 없이 빈 목록만 받아 "분석한 적 없음" 으로 보이고, 핸들러 쪽이
// 빠지면 쌓인 결과를 아무도 읽지 못한다. 둘 다 컴파일과 단위 테스트를 통과한다.
func TestSASTResultsAreWiredToSyncAndHandler(t *testing.T) {
	raw, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(raw)

	if !strings.Contains(src, "cicdrepo.NewPostgresSASTResultRepository(pool)") {
		t.Fatal("main.go 가 SAST 결과 저장소를 만들지 않는다")
	}
	for _, anchor := range []string{
		"runSyncUC := cicduc.NewSyncPipelineRuns(",
		"WithImageScans(pgImageScanRepo).\n\t\tWithSASTResults(pgSASTResultRepo)",
	} {
		i := strings.Index(src, anchor)
		if i < 0 {
			t.Fatalf("main.go 에서 %q 를 찾지 못했다", anchor)
		}
		// 배선 체인은 빈 줄에서 끝난다.
		chain := src[i:]
		if end := strings.Index(chain, "\n\n"); end >= 0 {
			chain = chain[:end]
		}
		if !strings.Contains(chain, "WithSASTResults(pgSASTResultRepo)") {
			t.Errorf("%q 체인에 WithSASTResults 가 없다", anchor)
		}
	}
}
