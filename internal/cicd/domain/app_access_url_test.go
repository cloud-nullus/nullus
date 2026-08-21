package domain

import "testing"

// 배포된 앱은 <앱이름>.<스택 도메인> 으로 열린다. 스캐폴딩이 만드는 HTTPRoute 의
// hostname 이 그 형태이므로(scaffold/renderer.go), 화면이 보여 주는 주소도 같은
// 규칙에서 나와야 한다 — 따로 적으면 한쪽이 바뀔 때 다른 쪽이 거짓말을 한다.
func TestAppAccessURL(t *testing.T) {
	cases := []struct {
		name   string
		app    string
		domain string
		want   string
	}{
		{"앱과 도메인이 있으면 https 주소", "sample-backend", "nullus.io", "https://sample-backend.nullus.io"},
		{"공백은 다듬는다", "  sample-frontend ", " nullus.io ", "https://sample-frontend.nullus.io"},
		// 도메인이 없는 스택은 밖에서 열리지 않는다. 없는 주소를 지어내면
		// 사용자는 열리지 않는 링크를 계속 누른다.
		{"도메인이 없으면 빈 값", "sample-backend", "", ""},
		{"앱 이름이 없으면 빈 값", "", "nullus.io", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := AppAccessURL(tc.app, tc.domain); got != tc.want {
				t.Fatalf("AppAccessURL(%q, %q) = %q, want %q", tc.app, tc.domain, got, tc.want)
			}
		})
	}
}

// 사용자가 도메인에 스킴을 넣어 두어도 주소가 깨지지 않아야 한다.
func TestAppAccessURL_StripsScheme(t *testing.T) {
	if got := AppAccessURL("api", "https://nullus.io"); got != "https://api.nullus.io" {
		t.Fatalf("got %q", got)
	}
}
