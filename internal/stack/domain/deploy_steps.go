package domain

import (
	"strings"
	"time"
)

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

// 진행 중으로 남을 수 있는 상태들. 이 상태의 스택은 어딘가에서 고루틴이 돌고
// 있다는 뜻이다.
var inFlightStates = map[DeploymentState]struct{}{
	StateValidating:  {},
	StateInstalling:  {},
	StateConfiguring: {},
	StateHealthCheck: {},
}

// IsInFlight 는 설치가 진행 중인 상태인지 본다.
func IsInFlight(state DeploymentState) bool {
	_, ok := inFlightStates[state]
	return ok
}

// StaleInstallThreshold 는 이만큼 아무 진전이 없으면 끊긴 것으로 본다.
//
// 한 스텝이 오래 걸릴 수 있다 — GitLab 은 helm --wait 타임아웃만 15분이다. 그
// 동안에는 행이 갱신되지 않으므로 여유를 크게 잡는다. 너무 짧게 잡으면 살아 있는
// 설치를 실패로 표시해 버린다.
const StaleInstallThreshold = 30 * time.Minute

// IsStaleInstall 은 진행 중이라고 표시돼 있지만 실제로는 끊긴 설치인지 본다.
//
// 설치는 API 프로세스 안의 고루틴이 돌린다. 파드가 교체되면 그 고루틴이 사라지고,
// 아무도 실패를 기록하지 않는다 — 스택은 installing 인 채로 영원히 남는다. 그
// 상태에서는 이어서 진행(continue)도 막히므로(failed/pending 만 허용) 사용자에게는
// 지우고 다시 까는 길밖에 없다. 2026-08-20 운영에서 실제로 그렇게 갇혔다.
//
// 살아 있는 설치와 구분할 방법은 시간뿐이다. 여러 레플리카가 도는 환경에서는
// "재시작했으니 죽었다" 고 단정할 수 없다.
func IsStaleInstall(state DeploymentState, updatedAt, now time.Time) bool {
	if !IsInFlight(state) {
		return false
	}
	if updatedAt.IsZero() {
		return false
	}
	return now.Sub(updatedAt) >= StaleInstallThreshold
}
