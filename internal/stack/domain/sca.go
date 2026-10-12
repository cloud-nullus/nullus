package domain

import "strings"

// 서드파티 의존성 취약점 스캔(SCA) — nullus-plan#79, 설계 docs/11_기능설계/Nullus_의존성_취약점_스캔_설계.md.
//
// 선택지는 Trivy 하나다. 따로 설치하는 것이 없다 — 파이프라인의 의존성 스캔 단계(trivy fs)가
// 스택의 **Trivy 서버**에 client 로 붙어 취약점 DB 를 서버에서 쓴다. 이미지 스캐너와 같은 서버라
// 둘 중 하나라도 고르면 서버가 서고, 둘 다 고르면 서버는 하나다.

// HasSCA 는 스택 Trivy 서버로 의존성을 스캔하는 선택인지 본다. 외부(external) 선택은 스택 안에
// 서버를 세우지 않는다 — 이미지 스캐너·SAST 와 같은 판단이다.
func HasSCA(sel ToolSelection) bool {
	return sel.Enabled && !strings.EqualFold(strings.TrimSpace(sel.Version), "external")
}

// HasTrivyServer 는 스택에 Trivy 서버를 세우는지 본다 — 이미지 스캐너나 의존성 스캔 중 하나라도
// 골랐으면 선다. 설치 단계(installing_trivy)·계획·모니터링 목록이 같은 답을 보도록 이 함수 하나를 쓴다.
func HasTrivyServer(cfg StackConfig) bool {
	return HasImageScanner(cfg.Security.ImageScanner) || HasSCA(cfg.Security.SCA)
}

// TrivyServerSelection 은 Trivy 서버의 버전·계획을 정하는 선택이다. 이미지 스캐너를 골랐으면 그것이
// 기준이고, 의존성 스캔만 골랐으면 그 선택이 서버를 대표한다. 서버가 서지 않으면 빈 선택이다.
//
// 계획(applied_resource_overrides)은 이 선택을 이미지 스캐너 슬롯(security.imageScanner)에 싣는다 —
// 설치 단계는 슬롯 하나로 계획을 찾으므로, 의존성 스캔만 고른 스택도 같은 자리로 서버 크기를 받는다.
func TrivyServerSelection(cfg StackConfig) ToolSelection {
	if HasImageScanner(cfg.Security.ImageScanner) {
		return cfg.Security.ImageScanner
	}
	if HasSCA(cfg.Security.SCA) {
		sel := cfg.Security.SCA
		if strings.TrimSpace(sel.Name) == "" {
			sel.Name = "Trivy"
		}
		return sel
	}
	return ToolSelection{}
}
