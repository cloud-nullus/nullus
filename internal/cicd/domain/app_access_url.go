package domain

import "strings"

// AppAccessURL 은 배포된 앱의 외부 접속 주소다.
//
// 스캐폴딩이 만드는 HTTPRoute 의 hostname 과 같은 규칙에서 나온다
// (scaffold/renderer.go 의 renderHTTPRoute: `<앱>.<도메인>`). 화면이 보여 주는
// 주소를 따로 적으면 한쪽이 바뀔 때 다른 쪽이 거짓말을 한다.
//
// 스택에 접근 도메인이 없으면 앱은 클러스터 안에서만 닿는다. 그때는 빈 값을
// 돌려준다 — 없는 주소를 지어내면 사용자는 열리지 않는 링크를 계속 누른다.
func AppAccessURL(appName, accessDomain string) string {
	app := strings.TrimSpace(appName)
	domain := strings.TrimSpace(accessDomain)
	if app == "" || domain == "" {
		return ""
	}

	// 사용자가 도메인에 스킴을 적어 두었을 수 있다. 그대로 붙이면
	// https://api.https://nullus.io 가 된다.
	domain = strings.TrimPrefix(domain, "https://")
	domain = strings.TrimPrefix(domain, "http://")
	domain = strings.Trim(strings.TrimSpace(domain), "/")
	if domain == "" {
		return ""
	}

	return "https://" + app + "." + domain
}
