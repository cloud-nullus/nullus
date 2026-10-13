package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sharednotify "github.com/cloud-nullus/draft/internal/shared/notification"
)

func source(name, severity string, extra map[string]string) SourceAlert {
	labels := map[string]string{"alertname": name, "severity": severity}
	for k, v := range extra {
		labels[k] = v
	}
	return SourceAlert{
		Fingerprint: "fp-" + name,
		Labels:      labels,
		Annotations: map[string]string{
			"summary":     "Filesystem is predicted to run out of space within the next 4 hours.",
			"description": "Filesystem on /dev/vda1 at 10.0.0.1:9100 has only 2.1% available space left.",
			"runbook_url": "https://runbooks.prometheus-operator.dev/runbooks/node/nodefilesystemspacefillingup",
		},
		StartsAt: time.Date(2026, 10, 13, 1, 0, 0, 0, time.UTC),
	}
}

var testStack = MonitoredStack{ID: "stk_1", OrgID: "org-1", Name: "gitlab-sca", Namespace: "nullus-gitlab-sca", ClusterID: "cl-1"}

// 기본 거르기 — 심각(critical)만 받고, 어디서나 뜨는 잡음은 뺀다.
func TestDefaultAlertFilter(t *testing.T) {
	f := DefaultAlertFilter()
	cases := []struct {
		alert SourceAlert
		want  bool
	}{
		{source("NodeFilesystemAlmostOutOfSpace", "critical", nil), true},
		{source("KubePersistentVolumeFillingUp", "critical", nil), true},
		{source("KubePodCrashLooping", "warning", nil), false},
		{source("TargetDown", "warning", nil), false},
		{source("Watchdog", "none", nil), false},
		{source("InfoInhibitor", "none", nil), false},
		// 컨트롤 플레인을 긁지 못하는 클러스터(kind · kubeadm 기본 · 관리형)에서는 늘 뜬다 — 2026-10-13
		// 로컬 kind 에서 심각 4건이 전부 이것이었다.
		{source("KubeControllerManagerDown", "critical", nil), false},
		{source("KubeSchedulerDown", "critical", nil), false},
		{source("KubeProxyDown", "critical", nil), false},
		{source("etcdInsufficientMembers", "critical", nil), false},
		{source("etcdMembersDown", "critical", nil), false},
		{source("", "critical", nil), false},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, f.Accept(tc.alert), tc.alert.Labels["alertname"])
	}
}

func TestNewStackAlert_TakesWhatPeopleRead(t *testing.T) {
	now := time.Date(2026, 10, 13, 2, 0, 0, 0, time.UTC)
	a := NewStackAlert(testStack, source("NodeFilesystemAlmostOutOfSpace", "critical", map[string]string{"instance": "10.0.0.1:9100"}), now)

	assert.Equal(t, "org-1", a.OrgID)
	assert.Equal(t, "stk_1", a.StackID)
	assert.Equal(t, "gitlab-sca", a.StackName)
	assert.Equal(t, "fp-NodeFilesystemAlmostOutOfSpace", a.Fingerprint)
	assert.Equal(t, "NodeFilesystemAlmostOutOfSpace", a.AlertName)
	assert.Equal(t, "critical", a.Severity)
	assert.Contains(t, a.Summary, "run out of space")
	assert.Contains(t, a.Description, "2.1% available")
	assert.Contains(t, a.RunbookURL, "runbooks.prometheus-operator.dev")
	assert.Equal(t, "10.0.0.1:9100", a.Labels["instance"])
	assert.Equal(t, source("x", "critical", nil).StartsAt, a.StartedAt)
	assert.Equal(t, now, a.LastSeenAt)
	assert.Nil(t, a.ResolvedAt)
}

// 대상은 사람이 찾아갈 수 있는 것 중 가장 구체적인 것이다.
func TestStackAlert_Target(t *testing.T) {
	a := NewStackAlert(testStack, source("KubePodNotReady", "critical", map[string]string{
		"namespace": "nullus-gitlab-sca", "pod": "gitlab-webservice-0", "instance": "10.0.0.1:8080",
	}), time.Now())
	assert.Equal(t, "nullus-gitlab-sca/gitlab-webservice-0", a.Target())

	a = NewStackAlert(testStack, source("NodeFilesystemAlmostOutOfSpace", "critical", map[string]string{"instance": "10.0.0.1:9100"}), time.Now())
	assert.Equal(t, "10.0.0.1:9100", a.Target())

	a = NewStackAlert(testStack, source("KubePersistentVolumeFillingUp", "critical", map[string]string{
		"namespace": "nullus-gitlab-sca", "persistentvolumeclaim": "data-postgresql-0",
	}), time.Now())
	assert.Equal(t, "nullus-gitlab-sca/data-postgresql-0", a.Target())
}

func TestStackAlert_FiredEvent(t *testing.T) {
	a := NewStackAlert(testStack, source("NodeFilesystemAlmostOutOfSpace", "critical", map[string]string{"instance": "10.0.0.1:9100"}), time.Now())

	e := a.FiredEvent(false)
	assert.Equal(t, sharednotify.EventAlertFired, e.Type)
	assert.Equal(t, sharednotify.SeverityCritical, e.Severity)
	assert.Equal(t, "org-1", e.OrgID)
	assert.Equal(t, "NodeFilesystemAlmostOutOfSpace — gitlab-sca", e.Title)
	assert.Contains(t, e.Summary, "run out of space")
	assert.Contains(t, e.Summary, "2.1% available")
	assert.Equal(t, a.StartedAt, e.OccurredAt)
	fields := map[string]string{}
	for _, f := range e.Fields {
		fields[f.Name] = f.Value
	}
	assert.Equal(t, "gitlab-sca", fields["스택"])
	assert.Equal(t, "10.0.0.1:9100", fields["대상"])
	assert.Contains(t, fields["런북"], "runbooks.prometheus-operator.dev")

	repeat := a.FiredEvent(true)
	assert.Equal(t, "계속 발생 중: NodeFilesystemAlmostOutOfSpace — gitlab-sca", repeat.Title)
}

func TestStackAlert_ResolvedEvent(t *testing.T) {
	a := NewStackAlert(testStack, source("KubePodNotReady", "critical", nil), time.Now())
	at := a.StartedAt.Add(42 * time.Minute)
	require.True(t, a.Resolve(at))
	assert.False(t, a.Resolve(at.Add(time.Minute)), "이미 해제된 경보는 다시 해제하지 않는다")

	e := a.ResolvedEvent()
	assert.Equal(t, sharednotify.EventAlertResolved, e.Type)
	assert.Equal(t, sharednotify.SeverityInfo, e.Severity)
	assert.Equal(t, "해제: KubePodNotReady — gitlab-sca", e.Title)
	assert.Contains(t, e.Summary, "42m0s")
	assert.Equal(t, at, e.OccurredAt)
}

// 해제는 유예가 지난 뒤에 알게 된다 — 이어진 시간은 마지막으로 본 때까지로 센다(해제를 안 시각이 아니라).
func TestStackAlert_ResolvedEventDurationUsesLastSeen(t *testing.T) {
	a := NewStackAlert(testStack, source("KubePodNotReady", "critical", nil), time.Now())
	a.LastSeenAt = a.StartedAt.Add(8 * time.Minute)
	require.True(t, a.Resolve(a.StartedAt.Add(11*time.Minute)))
	assert.Contains(t, a.ResolvedEvent().Summary, "약 8m0s")
}

func TestStackAlert_SummaryFallback(t *testing.T) {
	src := source("CustomCritical", "critical", nil)
	src.Annotations = nil
	a := NewStackAlert(testStack, src, time.Now())
	assert.NotEmpty(t, a.FiredEvent(false).Summary)
}
