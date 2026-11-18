package domain

import (
	"strconv"
	"strings"
)

// NexusH2FirstAppVersion 은 Nexus 가 OrientDB 를 버린 첫 버전이다.
//
// 3.71.0 부터는 H2 · PostgreSQL 만 연다. OrientDB 로 쌓인 데이터는 3.70.x 이하에서
// Sonatype 마이그레이터로 옮긴 뒤에야 올릴 수 있다 — 이미지 태그만 바꾸면 파드가
// 데이터베이스를 열지 못하고 서지 않는다.
const NexusH2FirstAppVersion = "3.71.0"

// NexusImageTagForRelease 는 Nexus 릴리스에 쓸 이미지 태그를 정한다.
//
// deployed 는 지금 떠 있는 릴리스의 태그다(새 설치면 빈 값). OrientDB 시대(3.71.0
// 미만) 태그로 떠 있으면 그 태그를 그대로 돌려준다 — 재시도 · 도구 추가가 설치 단계를
// 다시 돌릴 때 기존 데이터를 열 수 없는 버전으로 올리지 않게 한다. 그 밖에는
// NexusAppVersion 이다.
func NexusImageTagForRelease(deployed string) string {
	if cmp, ok := compareDottedVersions(deployed, NexusH2FirstAppVersion); ok && cmp < 0 {
		return strings.TrimSpace(deployed)
	}
	return NexusAppVersion
}

// compareDottedVersions 는 "3.64.0" 같은 점 구분 숫자 버전을 비교한다.
// 어느 한쪽이라도 숫자로 읽을 수 없으면 ok 가 false 다.
func compareDottedVersions(a, b string) (int, bool) {
	pa, ok := parseDottedVersion(a)
	if !ok {
		return 0, false
	}
	pb, ok := parseDottedVersion(b)
	if !ok {
		return 0, false
	}
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var x, y int
		if i < len(pa) {
			x = pa[i]
		}
		if i < len(pb) {
			y = pb[i]
		}
		if x != y {
			if x > y {
				return 1, true
			}
			return -1, true
		}
	}
	return 0, true
}

func parseDottedVersion(v string) ([]int, bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil, false
	}
	parts := strings.Split(v, ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return nil, false
		}
		out = append(out, n)
	}
	return out, true
}
