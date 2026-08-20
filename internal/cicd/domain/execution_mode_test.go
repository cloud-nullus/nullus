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
			// 빌드가 없는 파이프라인은 위임할 것이 없다. 기존 매니페스트 적용
			// 경로를 그대로 둔다.
			name:     "Dockerfile 이 없으면 위임할 빌드가 없다",
			pipeline: &Pipeline{StackID: "stk-1"},
			want:     false,
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
