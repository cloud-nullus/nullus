package domain

import (
	"fmt"
	"sort"
	"time"
)

// 보존 정책. 설계 §4.5 (nullus-plan#75)
//
// 순수 함수로 둔 이유: 잘못 지우면 되돌릴 수 없다. DB 없이 경계 조건 전부를
// 검증할 수 있어야 한다.

// RunSummary 는 보존 판단에 필요한 최소 정보다.
type RunSummary struct {
	ID         string
	CreatedAt  time.Time
	TotalBytes int64
	Status     Status
}

type RetentionPolicy struct {
	Daily   int
	Weekly  int
	Monthly int
	// MaxTotalBytes 가 0 이면 총량 상한을 적용하지 않는다.
	MaxTotalBytes int64
}

// DefaultRetentionPolicy 는 설계 §4.5 의 일7/주4/월3 이다.
func DefaultRetentionPolicy() RetentionPolicy {
	return RetentionPolicy{Daily: 7, Weekly: 4, Monthly: 3}
}

// hasArtifacts 는 지울 산출물이 있는 실행인지 판단한다.
//
// 실패한 실행에는 산출물이 없다. 이력은 "언제 백업이 끊겼나" 의 근거이므로
// 삭제 대상으로 잡지 않는다.
func hasArtifacts(s Status) bool {
	return s == StatusSucceeded || s == StatusPartial
}

// SelectForDeletion 은 산출물을 지울 실행 ID 를 돌려준다. 이력 행 자체는 남는다.
//
// 어떤 경우에도 가장 최근 성공본은 남긴다 — 보존 정책이 백업을 0 개로
// 만들면 정책이 아니라 사고다.
func (p RetentionPolicy) SelectForDeletion(runs []RunSummary) []string {
	candidates := make([]RunSummary, 0, len(runs))
	for _, r := range runs {
		if hasArtifacts(r.Status) {
			candidates = append(candidates, r)
		}
	}
	if len(candidates) == 0 {
		return nil
	}

	// 최신순.
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].CreatedAt.After(candidates[j].CreatedAt)
	})

	newest := candidates[0].ID
	keep := map[string]bool{newest: true}

	markBuckets := func(limit int, key func(time.Time) string) {
		if limit <= 0 {
			return
		}
		seen := map[string]bool{}
		for _, r := range candidates {
			k := key(r.CreatedAt)
			if seen[k] {
				continue
			}
			seen[k] = true
			keep[r.ID] = true
			if len(seen) >= limit {
				return
			}
		}
	}

	markBuckets(p.Daily, func(t time.Time) string { return t.Format("2006-01-02") })
	markBuckets(p.Weekly, func(t time.Time) string {
		y, w := t.ISOWeek()
		return fmt.Sprintf("%04d-W%02d", y, w)
	})
	markBuckets(p.Monthly, func(t time.Time) string { return t.Format("2006-01") })

	// 총량 상한. 남기기로 한 것 중에서도 오래된 것부터 더 지운다.
	if p.MaxTotalBytes > 0 {
		var total int64
		for _, r := range candidates {
			if keep[r.ID] {
				total += r.TotalBytes
			}
		}
		for i := len(candidates) - 1; i >= 0 && total > p.MaxTotalBytes; i-- {
			r := candidates[i]
			if !keep[r.ID] || r.ID == newest {
				continue
			}
			delete(keep, r.ID)
			total -= r.TotalBytes
		}
	}

	out := make([]string, 0, len(candidates))
	for _, r := range candidates {
		if !keep[r.ID] {
			out = append(out, r.ID)
		}
	}
	return out
}
