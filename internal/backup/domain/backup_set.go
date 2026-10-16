package domain

import "sort"

// 백업 세트 — 플랫폼 전체를 한 번에 뜬다.
//
// 한 번의 백업 실행은 플랫폼 + 스택 하나다(§6.5). 스택이 여럿이면 실행마다 시각이 어긋나고, 스택 A 의
// full 복구가 플랫폼 DB 를 A 시점으로 되돌려 그 뒤에 만든 스택 B 의 기록을 지운다. 세트는 **플랫폼
// 한 번 + 스택마다 한 번**을 같은 SetID 로 묶는다. 복구는 플랫폼(platform_only) → 스택(stack_only)
// 순서로 되돌려 스택 복구가 플랫폼 DB 를 다시 덮지 않는다.
//
// 세트는 테이블이 아니다 — backup_runs.set_id 로 묶인 실행들이고 상태는 멤버에서 파생한다(SetStatus).
// 실행 하나가 실패해도 나머지는 계속 뜬다: "일부는 쓸 수 있는 세트" 가 "아무것도 없는 세트" 보다 낫다.

// StackRef 는 세트가 뜰 스택이다(스택 모듈이 알려 준다).
type StackRef struct {
	ID        string
	Namespace string
}

// SetMember 는 세트의 실행 하나다.
type SetMember struct {
	StackID   string
	Namespace string
	Mode      Mode
	Scope     []Component
}

// PlanBackupSet 은 세트의 실행 계획이다. 플랫폼이 먼저다 — 복구가 플랫폼 → 스택 순서라, 플랫폼 DB 가
// 스택 데이터보다 **오래된** 쪽이 안전하다(DB 가 모르는 저장소는 고아로 남지만, DB 가 아는 저장소가
// 없으면 파이프라인이 깨진다). 스택은 ID 순 — 같은 입력이면 같은 계획이다.
func PlanBackupSet(stacks []StackRef) []SetMember {
	members := []SetMember{{Mode: ModePlatformOnly, Scope: ModeComponents(ModePlatformOnly)}}
	sorted := append([]StackRef(nil), stacks...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })
	for _, s := range sorted {
		if s.ID == "" {
			continue
		}
		// 스택 금고(OpenBao KV)는 모드 분류상 플랫폼 쪽이지만 스택마다 배포된다 — 스택 멤버가 담는다.
		scope := append(ModeComponents(ModeStackOnly), ComponentOpenBaoKV)
		members = append(members, SetMember{StackID: s.ID, Namespace: s.Namespace, Mode: ModeStackOnly, Scope: scope})
	}
	return members
}

// SetStatus 는 멤버 실행들로 세트의 상태를 정한다.
func SetStatus(runs []*BackupRun) Status {
	if len(runs) == 0 {
		return StatusFailed
	}
	var ok, bad, partial int
	for _, r := range runs {
		switch r.Status {
		case StatusSucceeded:
			ok++
		case StatusPartial:
			partial++
		case StatusFailed:
			bad++
		default:
			return StatusRunning
		}
	}
	switch {
	case bad == len(runs):
		return StatusFailed
	case bad == 0 && partial == 0:
		return StatusSucceeded
	default:
		return StatusPartial
	}
}

// SetRestoreStep 은 세트 복구의 한 단계다.
type SetRestoreStep struct {
	BackupRunID string
	StackID     string
	Namespace   string
	Mode        RestoreMode
}

// PlanSetRestore 는 세트의 실행들을 되돌릴 순서로 늘어놓는다 — 플랫폼(platform_only) 먼저, 그 뒤 스택
// (stack_only, 네임스페이스는 백업 매니페스트의 것). 실패한 멤버는 산출물이 없으니 건너뛰고 ID 를
// 돌려준다. partial 은 쓸 수 있는 산출물이 있어 되돌린다.
func PlanSetRestore(runs []*BackupRun) (steps []SetRestoreStep, skipped []string) {
	var platform *BackupRun
	var stacks []*BackupRun
	for _, r := range runs {
		if r.Status == StatusFailed || r.Status == StatusPending || r.Status == StatusRunning {
			skipped = append(skipped, r.ID)
			continue
		}
		if r.StackID == "" {
			platform = r
		} else {
			stacks = append(stacks, r)
		}
	}
	sort.Slice(stacks, func(i, j int) bool { return stacks[i].StackID < stacks[j].StackID })
	if platform != nil {
		steps = append(steps, SetRestoreStep{BackupRunID: platform.ID, Mode: ModePlatformOnly})
	}
	for _, r := range stacks {
		ns, _ := r.Manifest["namespace"].(string)
		steps = append(steps, SetRestoreStep{BackupRunID: r.ID, StackID: r.StackID, Namespace: ns, Mode: ModeStackOnly})
	}
	return steps, skipped
}
