package rotation

import (
	"context"
	"errors"
	"testing"
)

// 회전 후 반영 전략의 핵심 — 소비 방식에 따라 재시작 필요 여부가 다르다.
//
// Runner 는 기동 시 config 를 1회만 렌더링하므로 Secret 이 갱신돼도 이미 떠 있는
// 파드는 이전 값을 붙들고 있다. 반면 ArgoCD 의 repository Secret 은 매 요청
// 시점에 읽으므로 재시작이 필요 없다.
func TestRestartPolicy_RequiresRestart(t *testing.T) {
	cases := []struct {
		provider string
		want     bool
		why      string
	}{
		{"gitlab-runner", true, "config.toml 을 기동 시 1회만 렌더링"},
		{"gitlab", true, "설정을 기동 시 읽음"},
		{"postgresql", true, "자격증명 변경 시 재기동 필요"},
		{"minio", true, "루트 자격 변경 시 재기동 필요"},
		{"argocd", false, "repository Secret 을 매 요청 시 읽음"},
		{"unknown-tool", false, "알 수 없으면 재시작하지 않는다 (보수적)"},
	}

	for _, tc := range cases {
		if got := RequiresRestart(tc.provider); got != tc.want {
			t.Errorf("RequiresRestart(%q) = %v, want %v (%s)", tc.provider, got, tc.want, tc.why)
		}
	}
}

// 대상 워크로드가 없으면 재시작은 no-op 이어야 한다.
func TestRestarter_SkipsWhenNoWorkload(t *testing.T) {
	r := &WorkloadRestarter{
		restart: func(_ context.Context, _, _, _ string) error {
			t.Fatal("워크로드가 없는데 재시작을 시도했습니다")
			return nil
		},
		lookup: func(_ context.Context, _, _ string) ([]WorkloadRef, error) {
			return nil, nil
		},
	}

	if err := r.RestartForProvider(context.Background(), "ns", "gitlab-runner"); err != nil {
		t.Fatalf("no-op 이어야 하는데 오류: %v", err)
	}
}

// 재시작이 필요 없는 provider 는 조회조차 하지 않는다.
func TestRestarter_SkipsWhenRestartNotRequired(t *testing.T) {
	r := &WorkloadRestarter{
		lookup: func(_ context.Context, _, _ string) ([]WorkloadRef, error) {
			t.Fatal("재시작이 불필요한 provider 를 조회했습니다")
			return nil, nil
		},
	}

	if err := r.RestartForProvider(context.Background(), "ns", "argocd"); err != nil {
		t.Fatalf("no-op 이어야 하는데 오류: %v", err)
	}
}

// 조회된 워크로드마다 재시작을 요청한다.
func TestRestarter_RestartsEachWorkload(t *testing.T) {
	var restarted []string
	r := &WorkloadRestarter{
		lookup: func(_ context.Context, _, _ string) ([]WorkloadRef, error) {
			return []WorkloadRef{
				{Kind: "deployment", Name: "gitlab-runner"},
				{Kind: "statefulset", Name: "gitlab-gitaly"},
			}, nil
		},
		restart: func(_ context.Context, _, kind, name string) error {
			restarted = append(restarted, kind+"/"+name)
			return nil
		},
	}

	if err := r.RestartForProvider(context.Background(), "ns", "gitlab-runner"); err != nil {
		t.Fatalf("재시작 실패: %v", err)
	}
	if len(restarted) != 2 {
		t.Fatalf("2개를 재시작해야 하는데 %d개: %v", len(restarted), restarted)
	}
}

// 일부 실패해도 나머지를 계속 시도하고 오류를 모아 돌려준다.
// 하나가 막혔다고 다른 워크로드가 옛 자격증명에 머물면 안 된다.
func TestRestarter_ContinuesOnPartialFailure(t *testing.T) {
	boom := errors.New("restart failed")
	var attempted int
	r := &WorkloadRestarter{
		lookup: func(_ context.Context, _, _ string) ([]WorkloadRef, error) {
			return []WorkloadRef{
				{Kind: "deployment", Name: "a"},
				{Kind: "deployment", Name: "b"},
			}, nil
		},
		restart: func(_ context.Context, _, _, name string) error {
			attempted++
			if name == "a" {
				return boom
			}
			return nil
		},
	}

	err := r.RestartForProvider(context.Background(), "ns", "gitlab")
	if err == nil {
		t.Fatal("오류가 전파되어야 합니다")
	}
	if attempted != 2 {
		t.Fatalf("실패 후에도 계속 시도해야 합니다: attempted=%d", attempted)
	}
}
