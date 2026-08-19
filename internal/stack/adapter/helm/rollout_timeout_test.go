package helm

import "testing"

// 기동에 오래 걸리는 릴리스는 기본 180초 안에 준비되지 못해 설치가 통째로
// 실패한다. 실제로 Jenkins 가 그랬다:
//
//	runtime readiness failed for jenkins: kubectl rollout status ...
//	Waiting for 1 pods to be ready... error: timed out waiting for the condition
//
// Jenkins 는 기동 시 init 컨테이너가 updates.jenkins.io 에서 플러그인을 받는다.
// SSO 를 위해 oic-auth(의존 플러그인이 여럿)를 더하면서 180초를 넘겼다.
func TestRolloutTimeoutFor(t *testing.T) {
	if got := rolloutTimeoutFor("jenkins"); got == defaultRolloutTimeout {
		t.Fatalf("Jenkins 는 플러그인 설치로 기본 시간을 넘긴다, got %s", got)
	}
	if got := rolloutTimeoutFor("gitlab"); got == defaultRolloutTimeout {
		t.Fatalf("GitLab 도 예외다, got %s", got)
	}
	if got := rolloutTimeoutFor("argo-cd"); got != defaultRolloutTimeout {
		t.Fatalf("그 밖의 릴리스는 기본값이어야 한다, got %s", got)
	}
	// 공백이 섞여도 같은 판단이어야 한다.
	if rolloutTimeoutFor("  jenkins ") == defaultRolloutTimeout {
		t.Fatal("공백 때문에 예외를 놓쳤다")
	}
}
