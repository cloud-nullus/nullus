package domain

import "strings"

// 설치가 밟는 단계의 순서. 진행률의 단일 출처다.
//
// 예전에는 화면(단계 5개)과 서버(손으로 적은 스텝→퍼센트 표)가 각자 값을 갖고
// 있었다. 표에는 provisioning_secrets·installing_postgresql·installing_gitea 처럼
// 실제로 밟는 스텝의 절반이 빠져 있어서, 그 스텝에서는 진행률이 0 으로 나왔다.
// 그러면 화면이 다른 근거로 값을 지어내고, 시크릿만 깔린 시점에 막대가 절반을
// 넘긴 것처럼 보였다.
//
// 순서는 오케스트레이터가 실제로 도는 순서와 같아야 한다 — 그쪽이 이 목록을
// 그대로 쓴다.
var InstallStepOrder = []string{
	"installing_cert_manager",
	"installing_prometheus_crds",
	"installing_metrics_server",
	"installing_openbao",
	"installing_external_secrets",
	"provisioning_secrets",
	"installing_postgresql",
	"installing_minio",
	"installing_object_storage_secret",
	"installing_object_storage_buckets",
	"installing_database_connection_check",
	"provisioning_sso",
	"installing_gitlab",
	"installing_gitea",
	"provisioning_gitea",
	"installing_harbor",
	"provisioning_harbor",
	"installing_nexus",
	"provisioning_nexus",
	"installing_argocd",
	"installing_runner",
	"installing_jenkins",
	"installing_prometheus",
	"installing_grafana",
	"installing_logging",
	"installing_log_search",
	"installing_opentelemetry",
	"installing_otel_collector",
	"installing_otel_agent",
	"installing_gateway",
	"integration_check",
}

// 설치 전후의 구간. 설치 스텝들이 그 사이를 균등하게 나눠 갖는다.
const (
	validateProgress       = 5
	installBandStart       = 5
	installBandEnd         = 90
	configuringProgress    = 93
	healthCheckProgress    = 96
	completedProgress      = 100
	deleteStartedProgress  = 5
	deleteReleaseProgress  = 45
	deleteManifestProgress = 75
)

// stepProgressOverrides 는 설치 스텝 목록 밖의 단계다.
var stepProgressOverrides = map[string]int{
	"validate":          validateProgress,
	"continue":          validateProgress,
	"configuring":       configuringProgress,
	"health_check":      healthCheckProgress,
	"completed":         completedProgress,
	"deleting_started":  deleteStartedProgress,
	"deleting_release":  deleteReleaseProgress,
	"deleting_manifest": deleteManifestProgress,
	"deleted":           completedProgress,
	"delete_failed":     completedProgress,
}

// StepProgress 는 그 스텝을 밟는 동안 보여 줄 진행률이다.
//
// 설치 스텝은 5~90 을 균등하게 나눠 갖는다. 스텝 하나가 끝나면 딱 그만큼 오른다 —
// 시간이 아니라 실제로 한 일에 맞춰 움직인다.
//
// 모르는 스텝은 -1 이다. 0 을 돌려주면 "아직 시작 전" 과 구분되지 않아, 화면이
// 진행률을 다른 근거로 지어내게 된다.
func StepProgress(step string) int {
	key := strings.TrimSpace(step)
	if key == "" {
		return -1
	}
	if value, ok := stepProgressOverrides[key]; ok {
		return value
	}
	for index, candidate := range InstallStepOrder {
		if candidate == key {
			return installStepProgress(index)
		}
	}
	return -1
}

// StepProgressCeiling 은 그 스텝이 끝났을 때 닿을 값이다.
//
// 화면은 이 값을 상한으로 삼아 스텝 안에서만 조금씩 움직인다 — 다음 스텝의
// 몫까지 미리 채우면 하지 않은 일을 한 것처럼 보여 준다.
func StepProgressCeiling(step string) int {
	key := strings.TrimSpace(step)
	if value, ok := stepProgressOverrides[key]; ok {
		return value
	}
	for index, candidate := range InstallStepOrder {
		if candidate == key {
			return installStepProgress(index + 1)
		}
	}
	return -1
}

func installStepProgress(index int) int {
	span := installBandEnd - installBandStart
	return installBandStart + (span * index / len(InstallStepOrder))
}

// StackProgress 는 저장된 스택 상태만으로 진행률을 되살린다.
//
// 새로고침하면 WebSocket 이 처음부터 다시 붙어 그동안의 진행률을 모른다. 예전에는
// 화면이 상태(installing 등)를 뭉뚱그린 표로 대신 채웠는데, 그 값은 스텝 기반
// 진행률과 달라서 **새로고침할 때마다 퍼센트가 튀었다**. 같은 계산을 여기서 한 번
// 더 해 주면 두 경로가 같은 값을 본다.
//
// 진행 중인 스텝이 있으면 그 스텝의 값이다. 없으면 마지막으로 끝낸 스텝까지는
// 갔다는 뜻이므로 그 스텝의 상한을 쓴다.
func StackProgress(state DeploymentState, currentStep, lastCompletedStep string) (progress, ceiling int) {
	if state == StateCompleted {
		return completedProgress, completedProgress
	}

	if value := StepProgress(currentStep); value >= 0 {
		return value, StepProgressCeiling(currentStep)
	}
	if done := StepProgressCeiling(lastCompletedStep); done >= 0 {
		return done, done
	}
	return 0, 0
}
