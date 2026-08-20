// Package runner 는 배포 실행을 스택의 CI 러너에게 넘긴다.
//
// docker.Builder 의 반대편이다. 그쪽은 플랫폼이 직접 빌드하는 장애 대응
// 경로이고, 이쪽은 스택 컴포넌트가 실행하는 일반 경로다 — CI 가 빌드해
// 레지스트리에 올리고 CD 도구가 클러스터에 반영한다.
package runner

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"github.com/cloud-nullus/draft/internal/cicd/adapter/kube"
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// Delegate 는 port.BuildDelegate 의 구현체다.
type Delegate struct {
	factory port.SCMBundleFactory
	tracker *kube.StepTracker
}

// NewDelegate 는 러너 위임기를 만든다.
//
// factory 는 스택마다 다른 CI 서버 주소·자격증명을 요청 시점에 조립한다.
// tracker 는 배포 화면에 진행 상황을 보여 준다(없어도 위임 자체는 된다).
func NewDelegate(factory port.SCMBundleFactory, tracker *kube.StepTracker) *Delegate {
	return &Delegate{factory: factory, tracker: tracker}
}

// DelegateBuild 는 러너의 실행을 시작시키고 그 실행의 주소를 돌려준다.
func (d *Delegate) DelegateBuild(ctx context.Context, opts port.DelegateBuildOpts) (string, error) {
	d.markRunning(opts.DeploymentID, opts.StepIndex)

	trigger, baseURL, err := d.resolveTrigger(ctx, opts.StackID)
	if err != nil {
		d.markFailed(opts.DeploymentID, opts.StepIndex, err.Error())
		return "", err
	}

	runURL := jobRunURL(baseURL, opts.JobName, opts.Branch)
	d.log(opts.DeploymentID, opts.StepIndex, "$ CI 실행 요청: %s (%s)", opts.JobName, opts.Branch)

	if err := trigger.TriggerBuild(ctx, opts.JobName, opts.Branch); err != nil {
		// job 이 없으면 프로비저닝이 끝나지 않은 것이다. 그 사실을 그대로 말해
		// 준다 — "trigger failed" 만으로는 무엇을 해야 하는지 알 수 없다.
		message := err.Error()
		if strings.Contains(message, "404") {
			message = fmt.Sprintf(
				"CI 서버에 %s job 이 없습니다. 파이프라인 프로비저닝이 끝나지 않았거나 job 이 지워졌습니다 — "+
					"파이프라인을 다시 프로비저닝하세요", opts.JobName)
		}
		d.log(opts.DeploymentID, opts.StepIndex, "error: %s", message)
		d.markFailed(opts.DeploymentID, opts.StepIndex, "CI 실행 요청 실패")
		return "", fmt.Errorf("trigger ci build: %s", message)
	}

	d.log(opts.DeploymentID, opts.StepIndex, "CI 가 실행을 받았습니다. 이후 빌드·배포는 %s 에서 진행됩니다", runURL)
	d.markSuccess(opts.DeploymentID, opts.StepIndex, "CI 러너에 실행을 넘겼습니다")
	slog.Info("ci build delegated", "job", opts.JobName, "branch", opts.Branch, "run_url", runURL)
	return runURL, nil
}

// resolveTrigger 는 이 스택의 CI 서버를 찾는다.
func (d *Delegate) resolveTrigger(ctx context.Context, stackID string) (port.CIBuildTrigger, string, error) {
	if d.factory == nil {
		return nil, "", fmt.Errorf("스택 연결이 배선돼 있지 않아 CI 러너를 찾을 수 없습니다")
	}
	bundle, err := d.factory.For(ctx, stackID)
	if err != nil {
		return nil, "", fmt.Errorf("스택 %s 의 CI 연결을 읽지 못했습니다: %w", stackID, err)
	}
	if bundle == nil || bundle.CITrigger == nil {
		return nil, "", fmt.Errorf(
			"스택 %s 에 실행을 넘길 CI 플랫폼이 없습니다. 스택에 Jenkins 가 설치되어 있고 자격증명이 준비되었는지 확인하세요",
			stackID)
	}
	return bundle.CITrigger, bundle.CIBaseURL, nil
}

// jobRunURL 은 사람이 열어 볼 실행 주소다. 주소를 모르면 빈 값이다 —
// 지어내면 열리지 않는 링크가 화면에 남는다.
func jobRunURL(baseURL, job, branch string) string {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" || strings.TrimSpace(job) == "" {
		return ""
	}
	path := base + "/job/" + url.PathEscape(strings.TrimSpace(job))
	if b := strings.TrimSpace(branch); b != "" {
		path += "/job/" + url.PathEscape(b)
	}
	return path + "/"
}

func (d *Delegate) log(deploymentID string, stepIndex int, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if d.tracker != nil && deploymentID != "" {
		d.tracker.AppendLog(deploymentID, stepIndex, msg)
	}
}

func (d *Delegate) markRunning(deploymentID string, stepIndex int) {
	if d.tracker != nil && deploymentID != "" {
		d.tracker.MarkRunning(deploymentID, stepIndex, "")
	}
}

func (d *Delegate) markSuccess(deploymentID string, stepIndex int, message string) {
	if d.tracker != nil && deploymentID != "" {
		d.tracker.MarkSuccess(deploymentID, stepIndex, message)
	}
}

func (d *Delegate) markFailed(deploymentID string, stepIndex int, message string) {
	if d.tracker != nil && deploymentID != "" {
		d.tracker.MarkFailed(deploymentID, stepIndex, message)
	}
}
