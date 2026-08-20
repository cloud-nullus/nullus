package domain

import "strings"

// stackNamespacePrefix 는 스택 네임스페이스가 플랫폼과 섞이지 않게 하는 접두사다.
const stackNamespacePrefix = "nullus-"

// maxNamespaceLength 는 쿠버네티스 네임스페이스(RFC1123 라벨)의 상한이다.
const maxNamespaceLength = 63

// DefaultStackNamespaceFor 는 스택 이름에서 기본 네임스페이스를 만든다.
//
// 예전 기본값은 DefaultStackNamespace("nullus") 하나였고, 그것은 플랫폼 자신이
// 사는 네임스페이스와 같았다. 그래서 두 가지가 동시에 깨졌다 —
//
//   - 설치: 스택의 PostgreSQL 릴리스(nullus-postgresql)가 플랫폼 차트가 이미
//     소유한 같은 이름의 리소스와 부딪혀 Helm 이 거부한다.
//   - 삭제: 스택 정리가 같은 네임스페이스를 훑으며 플랫폼 리소스를 지운다.
//     2026-08-20 에 실제로 nullus.io 가 통째로 내려갔다.
//
// 스택마다 자기 네임스페이스를 갖게 하면 두 경로 모두 애초에 만나지 않는다.
func DefaultStackNamespaceFor(stackName string) string {
	slug := sanitizeNamespaceLabel(stackName)
	if slug == "" {
		return stackNamespacePrefix + "stack"
	}

	ns := stackNamespacePrefix + slug
	if len(ns) > maxNamespaceLength {
		ns = ns[:maxNamespaceLength]
	}
	return strings.TrimRight(ns, "-")
}

// sanitizeNamespaceLabel 은 사람이 지은 이름을 RFC1123 라벨 조각으로 옮긴다.
func sanitizeNamespaceLabel(value string) string {
	lowered := strings.ToLower(strings.TrimSpace(value))

	var b strings.Builder
	lastHyphen := false
	for _, r := range lowered {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			lastHyphen = false
		default:
			// 공백·밑줄·점 등은 전부 하이픈 하나로 접는다.
			if !lastHyphen && b.Len() > 0 {
				b.WriteByte('-')
				lastHyphen = true
			}
		}
	}

	return strings.Trim(b.String(), "-")
}
