package domain

import "strings"

// HasSAST 는 스택 안에 정적 분석(SAST) 서버를 세우는 선택인지 본다.
//
// 이미지 스캐너와 같은 규칙이다 — 슬롯의 선택지는 SonarQube 하나뿐이라 고른 사실
// 자체가 곧 설치 여부이고, 외부(external) 선택은 스택 안에 서지 않는다.
func HasSAST(sel ToolSelection) bool {
	return sel.Enabled && !strings.EqualFold(strings.TrimSpace(sel.Version), "external")
}
