// Package postgres 는 PostgreSQL 논리 백업/복원 어댑터다.
//
// 설계: docs/11_기능설계/Nullus_백업복구_설계.md §3.1 (nullus-plan#75)
package postgres

import (
	"fmt"
	"strconv"
	"strings"
)

// pg_dump 는 서버 버전보다 낮으면 실패한다. 그런데 에어갭 번들에는
// PostgreSQL 이미지가 여러 버전 섞여 있다 — 14.8.0 / 17.2.0 / 17.5.0.
// 그래서 서버 버전을 조회해 그 이상의 클라이언트를 골라야 한다.
//
// 이 규칙을 테스트로 고정하는 이유: 잘못 고르면 백업이 **조용히 실패**하거나
// 부분 dump 를 남긴다 (§9 F6). 실패했다는 사실조차 늦게 드러난다.

// DumpImage 는 pg_dump/pg_restore 를 담은 컨테이너 이미지다.
type DumpImage struct {
	Ref     string // 예: docker.io/bitnamilegacy/postgresql:17.5.0-debian-12-r20
	Version string // 예: 17.5.0
}

// majorMinor 는 버전 문자열에서 (major, minor) 를 뽑는다.
//
// PostgreSQL 서버는 "17.5" 로, 이미지 태그는 "17.5.0-debian-12-r20" 으로
// 온다. 패치 이하는 pg_dump 호환성 판단에 쓰이지 않으므로 버린다.
func majorMinor(v string) (int, int, error) {
	v = strings.TrimSpace(v)
	if i := strings.IndexAny(v, "-+ "); i >= 0 {
		v = v[:i]
	}
	if v == "" {
		return 0, 0, fmt.Errorf("버전 문자열이 비어 있습니다")
	}
	parts := strings.Split(v, ".")
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("버전을 해석할 수 없습니다: %q", v)
	}
	minor := 0
	if len(parts) > 1 {
		// "17.5.0" 의 5, "17" 의 경우 0.
		if m, err := strconv.Atoi(parts[1]); err == nil {
			minor = m
		}
	}
	return major, minor, nil
}

// IsCompatible 은 클라이언트가 서버를 덤프할 수 있는지 판정한다.
//
// 기준은 **major 버전**이다. PostgreSQL 은 같은 major 안의 minor 차이를
// 호환으로 다루고, pg_dump 는 자기보다 낮거나 같은 major 의 서버를 덤프한다.
//
// 이 규칙을 minor 까지 좁혀 잡았다가 리허설에서 걸렸다 — 클라이언트 18.3 /
// 서버 18.4 조합이 막혔다. 서버가 마이너 패치를 받는 것만으로 백업이 멈추면,
// 막으려던 것과 정반대 방향의 사고가 된다.
func IsCompatible(clientVersion, serverVersion string) (bool, error) {
	cMaj, _, err := majorMinor(clientVersion)
	if err != nil {
		return false, fmt.Errorf("클라이언트 버전 해석 실패: %w", err)
	}
	sMaj, _, err := majorMinor(serverVersion)
	if err != nil {
		return false, fmt.Errorf("서버 버전 해석 실패: %w", err)
	}
	return cMaj >= sMaj, nil
}

// SelectDumpImage 는 서버를 덤프할 수 있는 이미지 중 하나를 고른다.
//
// 규칙: 서버 major 이상인 것 중 **가장 낮은 major**, 그 안에서 **가장 높은
// minor**. 낮은 major 를 고르는 것은 산출물이 그 환경 밖에서도 열리게 하기
// 위해서이고, 높은 minor 를 고르는 것은 같은 major 안에서는 어차피 호환이라
// 버그 수정이 더 들어간 쪽이 낫기 때문이다.
func SelectDumpImage(serverVersion string, candidates []DumpImage) (DumpImage, error) {
	sMaj, _, err := majorMinor(serverVersion)
	if err != nil {
		return DumpImage{}, fmt.Errorf("서버 버전 해석 실패: %w", err)
	}

	var best DumpImage
	var bestMaj, bestMin int
	found := false

	for _, c := range candidates {
		cMaj, cMin, err := majorMinor(c.Version)
		if err != nil {
			continue
		}
		if cMaj < sMaj {
			continue // major 가 낮으면 pg_dump 가 거부한다
		}
		switch {
		case !found, cMaj < bestMaj, cMaj == bestMaj && cMin > bestMin:
			best, bestMaj, bestMin, found = c, cMaj, cMin, true
		}
	}

	if !found {
		return DumpImage{}, fmt.Errorf(
			"서버 major 버전 %d 이상의 pg_dump 이미지가 번들에 없습니다. 에어갭 번들에 추가해야 합니다", sMaj)
	}
	return best, nil
}
