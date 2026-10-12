package domain

import "testing"

// 스택에 묶인 파이프라인의 이미지는 스택의 CI 러너가 만든다. 플랫폼이 직접
// 만들려 하면 API 파드 안에서 git·docker 를 찾다 죽는다 — 그 파드에는 도커
// 데몬이 없고, 있을 이유도 없다.
func TestPipeline_DelegatesBuildToRunner(t *testing.T) {
	cases := []struct {
		name     string
		pipeline *Pipeline
		want     bool
	}{
		{
			name:     "스택에 묶이고 Dockerfile 이 있으면 러너가 맡는다",
			pipeline: &Pipeline{StackID: "stk-1", DockerfilePath: "Dockerfile"},
			want:     true,
		},
		{
			name: "긴급모드를 명시하면 플랫폼이 직접 만든다",
			pipeline: &Pipeline{
				StackID:        "stk-1",
				DockerfilePath: "Dockerfile",
				ExecutionMode:  ExecutionModeEmergencyDirect,
			},
			want: false,
		},
		{
			// 스택이 없으면 위임할 러너 자체가 없다.
			name:     "스택이 없으면 위임하지 않는다",
			pipeline: &Pipeline{DockerfilePath: "Dockerfile"},
			want:     false,
		},
		{
			// 스택에 묶였으면 Dockerfile 경로가 비어 있어도 러너가 실행한다. 플랫폼이
			// 매니페스트를 직접 적용하면 CD 도구와 서로 덮어쓴다 — API 로 Dockerfile
			// 경로 없이 만든 파이프라인이 그 경로로 새고 있었다.
			name:     "Dockerfile 경로가 비어 있어도 스택에 묶였으면 러너가 맡는다",
			pipeline: &Pipeline{StackID: "stk-1"},
			want:     true,
		},
		{
			name:     "nil 은 위임하지 않는다",
			pipeline: nil,
			want:     false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.pipeline.DelegatesBuildToRunner(); got != tc.want {
				t.Fatalf("DelegatesBuildToRunner() = %v, want %v", got, tc.want)
			}
		})
	}
}
