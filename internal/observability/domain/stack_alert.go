package domain

import (
	"fmt"
	"strings"
	"time"

	sharednotify "github.com/cloud-nullus/draft/internal/shared/notification"
)

// 스택 경보(nullus-plan#87). 스택마다 설치된 Prometheus 가 kube-prometheus-stack 기본 규칙을 평가하고,
// Alertmanager 가 활성 경보를 들고 있다. 플랫폼은 그것을 주기적으로 가져와 심각한 것만 조직의 알림
// 채널로 보낸다 — 규칙을 따로 쓰지 않는다. 설계: docs/11_기능설계/Nullus_스택_경보_전달_설계.md.

// MonitoredStack 은 경보를 가져올 스택이다(설치 완료 · 모니터링 수집을 고른 스택).
type MonitoredStack struct {
	ID        string
	OrgID     string
	Name      string
	Namespace string
	ClusterID string
}

// SourceAlert 는 Alertmanager 의 활성 경보 하나다.
type SourceAlert struct {
	Fingerprint string
	Labels      map[string]string
	Annotations map[string]string
	StartsAt    time.Time
	// Suppressed 는 침묵(silence) · 억제(inhibition)된 경보다. 알리지 않지만 "보인 것" 이다 — 침묵을
	// 걸었다고 해제로 읽으면 해제 알림이 나가고 침묵이 끝날 때 발생 알림이 다시 나간다.
	Suppressed bool
}

// AlertFilter 는 알림으로 보낼 경보를 고른다.
type AlertFilter struct {
	severities map[string]bool
	excluded   map[string]bool
	prefixes   []string
}

// DefaultAlertFilter 는 심각(critical)만 받고, 어디서나 뜨는 잡음을 뺀다.
//
//   - Watchdog · InfoInhibitor: 경보 경로가 살아 있음을 알리는 상시 경보와 억제용 경보다.
//   - 컨트롤 플레인 수집 실패: kube-prometheus-stack 은 controller-manager · scheduler · kube-proxy · etcd 를
//     kube-system Service 로 긁는데, kind · kubeadm 기본(127.0.0.1 바인드) · 관리형(EKS · GKE · AKS)
//     클러스터에서는 닿지 않아 늘 "Down" 이 뜬다. 2026-10-13 로컬 kind 에서 심각 경보 4건이 전부
//     이것이었다. 컨트롤 플레인이 실제로 죽으면 API 서버 · 노드 경보가 함께 뜬다.
func DefaultAlertFilter() AlertFilter {
	return AlertFilter{
		severities: map[string]bool{"critical": true},
		excluded: map[string]bool{
			"Watchdog":                  true,
			"InfoInhibitor":             true,
			"KubeControllerManagerDown": true,
			"KubeSchedulerDown":         true,
			"KubeProxyDown":             true,
		},
		prefixes: []string{"etcd"},
	}
}

// Accept 는 그 경보를 알림으로 보낼지다.
func (f AlertFilter) Accept(a SourceAlert) bool {
	name := strings.TrimSpace(a.Labels["alertname"])
	if name == "" || f.excluded[name] {
		return false
	}
	for _, p := range f.prefixes {
		if strings.HasPrefix(name, p) {
			return false
		}
	}
	return f.severities[strings.ToLower(strings.TrimSpace(a.Labels["severity"]))]
}

// StackAlert 는 플랫폼이 기록하는 스택 경보 하나다 — 발생부터 해제까지.
type StackAlert struct {
	ID          string
	OrgID       string
	StackID     string
	StackName   string
	Fingerprint string
	AlertName   string
	Severity    string
	Summary     string
	Description string
	RunbookURL  string
	Labels      map[string]string

	StartedAt      time.Time
	LastSeenAt     time.Time
	ResolvedAt     *time.Time
	LastNotifiedAt *time.Time
}

// NewStackAlert 는 가져온 경보를 기록할 모양으로 옮긴다.
func NewStackAlert(stack MonitoredStack, a SourceAlert, now time.Time) *StackAlert {
	labels := make(map[string]string, len(a.Labels))
	for k, v := range a.Labels {
		labels[k] = v
	}
	return &StackAlert{
		OrgID:       stack.OrgID,
		StackID:     stack.ID,
		StackName:   stack.Name,
		Fingerprint: a.Fingerprint,
		AlertName:   a.Labels["alertname"],
		Severity:    strings.ToLower(a.Labels["severity"]),
		Summary:     strings.TrimSpace(a.Annotations["summary"]),
		Description: strings.TrimSpace(a.Annotations["description"]),
		RunbookURL:  strings.TrimSpace(a.Annotations["runbook_url"]),
		Labels:      labels,
		StartedAt:   a.StartsAt,
		LastSeenAt:  now,
	}
}

// Target 은 사람이 찾아갈 수 있는 것 중 가장 구체적인 것이다(파드 · PVC · 노드 · 인스턴스 · 잡).
func (a *StackAlert) Target() string {
	ns := a.Labels["namespace"]
	for _, key := range []string{"pod", "persistentvolumeclaim", "deployment", "statefulset", "daemonset"} {
		if v := a.Labels[key]; v != "" {
			if ns != "" {
				return ns + "/" + v
			}
			return v
		}
	}
	for _, key := range []string{"node", "instance", "job", "service"} {
		if v := a.Labels[key]; v != "" {
			return v
		}
	}
	return ns
}

// Resolve 는 해제를 기록한다. 이미 해제됐으면 false 다.
func (a *StackAlert) Resolve(at time.Time) bool {
	if a.ResolvedAt != nil {
		return false
	}
	a.ResolvedAt = &at
	return true
}

// FiredEvent 는 발생 알림이다. repeat 이면 계속 발생 중이라는 다시 알림이다.
func (a *StackAlert) FiredEvent(repeat bool) sharednotify.Event {
	title := fmt.Sprintf("%s — %s", a.AlertName, a.StackName)
	if repeat {
		title = "계속 발생 중: " + title
	}
	summary := strings.TrimSpace(strings.Join(nonEmpty(a.Summary, a.Description), "\n"))
	if summary == "" {
		summary = "스택 Prometheus 의 규칙이 심각 경보를 냈습니다."
	}
	return sharednotify.Event{
		Type: sharednotify.EventAlertFired, OrgID: a.OrgID, Severity: sharednotify.SeverityCritical,
		Title: title, Summary: summary, Fields: a.fields(), OccurredAt: a.StartedAt,
	}
}

// ResolvedEvent 는 해제 알림이다. 이어진 시간은 마지막으로 본 때까지다 — 해제는 유예가 지난 뒤에 알게
// 되므로 해제를 안 시각으로 세면 유예만큼 길게 보인다.
func (a *StackAlert) ResolvedEvent() sharednotify.Event {
	at := time.Now()
	if a.ResolvedAt != nil {
		at = *a.ResolvedAt
	}
	end := a.LastSeenAt
	if end.IsZero() || end.Before(a.StartedAt) || end.After(at) {
		end = at
	}
	return sharednotify.Event{
		Type: sharednotify.EventAlertResolved, OrgID: a.OrgID, Severity: sharednotify.SeverityInfo,
		Title:      fmt.Sprintf("해제: %s — %s", a.AlertName, a.StackName),
		Summary:    fmt.Sprintf("약 %s 동안 이어진 경보가 해제됐습니다.", end.Sub(a.StartedAt).Round(time.Second)),
		Fields:     a.fields(),
		OccurredAt: at,
	}
}

func (a *StackAlert) fields() []sharednotify.Field {
	fields := []sharednotify.Field{{Name: "스택", Value: a.StackName}, {Name: "경보", Value: a.AlertName}}
	if t := a.Target(); t != "" {
		fields = append(fields, sharednotify.Field{Name: "대상", Value: t})
	}
	if a.RunbookURL != "" {
		fields = append(fields, sharednotify.Field{Name: "런북", Value: a.RunbookURL})
	}
	return fields
}

func nonEmpty(values ...string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			out = append(out, v)
		}
	}
	return out
}
