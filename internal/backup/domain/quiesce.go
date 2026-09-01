package domain

// 정지 백업(cold backup)의 정지/재개 규칙.
//
// 설계: docs/11_기능설계/Nullus_백업복구_설계.md §3.4 (nullus-plan#75)
//
// local-path 프로비저너는 CSI 스냅샷을 지원하지 않는다. 여러 볼륨을 같은
// 순간의 모습으로 얼리려면 쓰기를 멈추는 수밖에 없고, 그래서 백업 전에
// 워크로드를 0 으로 내렸다가 되돌린다.
//
// 이 파일이 순수 함수인 이유: 최악의 시나리오가 "백업하려다 서비스를 못
// 살리는 것"(§9 F3)이라, 클러스터 없이 전 경로를 테스트할 수 있어야 한다.

// Workload 는 정지 대상 후보다.
type Workload struct {
	Kind      string // Deployment | StatefulSet
	Namespace string
	Name      string
	Replicas  int32
}

// QuiesceTarget 은 실제로 멈출 워크로드와 되돌릴 값이다.
type QuiesceTarget struct {
	Kind      string
	Namespace string
	Name      string
	// OriginalReplicas 는 재개의 유일한 근거다. 매니페스트에 기록해 두지
	// 않으면 복구 시점에 몇으로 되돌려야 하는지 알 수 없다.
	OriginalReplicas int32
	ScaleDownTo      int32
}

// QuiescePlan 은 한 번의 정지 창에서 멈출 대상 전체다.
type QuiescePlan struct {
	Targets []QuiesceTarget
}

// NewQuiescePlan 은 워크로드 목록에서 정지 계획을 만든다.
//
// 이미 0 인 워크로드는 대상에서 제외한다. 그것을 "복원" 한다며 1 로 올리면
// 사용자가 의도적으로 꺼둔 것을 백업이 켜버린다.
func NewQuiescePlan(workloads []Workload) QuiescePlan {
	targets := make([]QuiesceTarget, 0, len(workloads))
	for _, w := range workloads {
		if w.Replicas <= 0 {
			continue
		}
		targets = append(targets, QuiesceTarget{
			Kind:             w.Kind,
			Namespace:        w.Namespace,
			Name:             w.Name,
			OriginalReplicas: w.Replicas,
			ScaleDownTo:      0,
		})
	}
	return QuiescePlan{Targets: targets}
}

// IsEmpty 는 멈출 것이 없는지 알려준다. 비어 있으면 정지 창 자체를 열지 않는다.
func (p QuiescePlan) IsEmpty() bool { return len(p.Targets) == 0 }

// ResumeOrder 는 재개 순서를 돌려준다 — 정지의 역순이다.
//
// 의존 관계(예: 앱 → DB)는 정지 순서에 반영되므로, 되살릴 때 그 역순을
// 따르면 의존 대상이 먼저 뜬다.
func (p QuiescePlan) ResumeOrder() []QuiesceTarget {
	out := make([]QuiesceTarget, len(p.Targets))
	for i, t := range p.Targets {
		out[len(p.Targets)-1-i] = t
	}
	return out
}
