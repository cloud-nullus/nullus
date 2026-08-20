package domain

import "strings"

// ToolAccessURL 은 스택 접속 도메인 위에 선 OSS 하나의 웹 주소다.
//
// 주소 규칙이 단일 출처여야 하는 이유는 화면이 여러 개이기 때문이다 — 스택 상세
// 화면, 모니터링 대시보드, 연결정보 안내가 모두 "Grafana 는 어디로 들어가는가"를
// 답해야 한다. 각자 조립하면 한 곳만 바뀌었을 때 조용히 갈라지고, 사용자는 화면마다
// 다른 주소를 본다.
//
// 스킴은 항상 https 다. 접속 도메인은 게이트웨이 TLS 리스너 뒤에 서고, 임베드
// 탭도 https 를 전제로 정규화한다 — 여기서 http 를 내려주면 브라우저가 혼합
// 콘텐츠로 막거나 리다이렉트 한 번을 더 타게 된다.
//
// 주소를 모르는 도구는 빈 문자열을 돌려준다. 그럴듯한 호스트를 지어내면 화면이
// 죽은 링크를 안내하게 되므로, 모를 때는 링크를 걸지 않는 편이 낫다.
func ToolAccessURL(toolName, accessDomain string) string {
	domain := strings.TrimSpace(accessDomain)
	if domain == "" {
		return ""
	}

	host := toolAccessHost(toolName)
	if host == "" {
		return ""
	}

	return "https://" + host + "." + domain
}

// toolAccessHost 는 제품 이름을 게이트웨이가 라우팅하는 호스트 라벨로 옮긴다.
//
// 설정에 담기는 이름은 사람이 고른 표기라 대소문자와 구분자가 제각각이다
// ("GitLab CE", "argo-cd", "Grafana Loki"). 정규화한 뒤 제품 단위로만 본다.
func toolAccessHost(toolName string) string {
	name := strings.Join(strings.Fields(
		strings.ReplaceAll(
			strings.ReplaceAll(strings.ToLower(strings.TrimSpace(toolName)), "_", " "),
			"-", " "),
	), " ")
	if name == "" {
		return ""
	}

	// 자체 UI 가 없는 도구(Loki / Tempo / 수집기)는 그 데이터를 보는 화면으로 보낸다.
	// 이 셋은 Grafana 의 데이터소스로만 존재하므로 자기 주소가 없다.
	switch {
	case strings.Contains(name, "loki"),
		strings.Contains(name, "tempo"),
		strings.Contains(name, "opentelemetry"),
		strings.Contains(name, "otel"),
		strings.Contains(name, "grafana"):
		return "grafana"
	}

	switch {
	case strings.Contains(name, "gitlab"):
		return "gitlab"
	case strings.Contains(name, "gitea"):
		return "gitea"
	case strings.Contains(name, "jenkins"):
		return "jenkins"
	case strings.Contains(name, "argocd"), strings.Contains(name, "argo cd"):
		return "argocd"
	case strings.Contains(name, "prometheus"):
		return "prometheus"
	case strings.Contains(name, "harbor"):
		return "harbor"
	case strings.Contains(name, "nexus"):
		return "nexus"
	case strings.Contains(name, "minio"):
		return "minio"
	case strings.Contains(name, "opensearch"):
		return "opensearch"
	// Elasticsearch 를 고르면 사용자가 들어가는 화면은 Kibana 다.
	case strings.Contains(name, "elasticsearch"), strings.Contains(name, "kibana"):
		return "kibana"
	case strings.Contains(name, "jaeger"):
		return "jaeger"
	case strings.Contains(name, "openbao"):
		return "openbao"
	}

	return ""
}
