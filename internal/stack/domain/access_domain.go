package domain

import (
	"fmt"
	"strings"
)

const (
	// maxAccessDomainLength 는 DNS 이름 전체의 상한이다.
	maxAccessDomainLength = 253
	// maxDomainLabelLength 는 점으로 나뉜 조각 하나의 상한이다.
	maxDomainLabelLength = 63
)

// ValidateAccessDomain 은 접속 도메인이 실제로 라우팅·발급에 쓸 수 있는 값인지 본다.
//
// 이 값 하나가 스택의 모든 주소가 된다 — HTTPRoute 의 hostname("gitlab.<도메인>"),
// 게이트웨이 인증서의 commonName 과 dnsNames("*.<도메인>"), 도구들의 redirect_uri.
// 그래서 형식이 깨지면 설치는 끝나도 아무것도 열리지 않는다.
//
// 2026-08-20 운영 스택은 access_domain 이 ".io" 로 저장돼 있었다. 만들어진
// 매니페스트는 hostname "jenkins..io", 인증서 dnsNames "*..io" 였다. 아무도
// 막지 않아서 설치 절차를 다 태운 뒤에야 드러났다.
func ValidateAccessDomain(accessDomain string) error {
	value := strings.TrimSpace(accessDomain)
	if value == "" {
		return fmt.Errorf("access_domain is required")
	}
	if len(value) > maxAccessDomainLength {
		return fmt.Errorf("access_domain 이 너무 깁니다 (%d자, 최대 %d자)", len(value), maxAccessDomainLength)
	}
	if strings.Contains(value, "://") || strings.ContainsAny(value, " \t/:*") {
		return fmt.Errorf("access_domain 은 스킴·경로·와일드카드 없이 도메인만 적습니다: %q", accessDomain)
	}

	labels := strings.Split(strings.ToLower(value), ".")
	// 한 라벨짜리 이름은 클러스터 밖에서 풀리지 않는다 — 최소 두 조각을 요구한다.
	if len(labels) < 2 {
		return fmt.Errorf("access_domain 은 점으로 구분된 이름이어야 합니다 (예: nullus.io): %q", accessDomain)
	}

	for _, label := range labels {
		if err := validateDomainLabel(label, accessDomain); err != nil {
			return err
		}
	}
	return nil
}

func validateDomainLabel(label, original string) error {
	if label == "" {
		return fmt.Errorf("access_domain 에 빈 조각이 있습니다 (점이 연속되었거나 앞뒤에 붙었습니다): %q", original)
	}
	if len(label) > maxDomainLabelLength {
		return fmt.Errorf("access_domain 의 조각 %q 가 너무 깁니다 (최대 %d자)", label, maxDomainLabelLength)
	}
	if strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
		return fmt.Errorf("access_domain 의 조각 %q 는 하이픈으로 시작하거나 끝날 수 없습니다", label)
	}
	for _, r := range label {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			continue
		}
		return fmt.Errorf("access_domain 에 쓸 수 없는 문자가 있습니다 (%q): %q", string(r), original)
	}
	return nil
}
