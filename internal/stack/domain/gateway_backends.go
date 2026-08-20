package domain

import "strings"

// GatewayBackend 는 도구 호스트가 가리켜야 할 클러스터 안 서비스다.
type GatewayBackend struct {
	Service string
	Port    int
}

// 도구별 백엔드. 게이트웨이 라우트가 어디로 보내야 하는지의 단일 출처다.
//
// 예전에는 서버와 설치 마법사가 각자 목록을 갖고 있었다. 마법사 쪽은 모르는
// 도구를 "<도구>-svc:80" 으로 만들었는데, 실제 서비스 이름과 포트는 도구마다
// 다르다(gitea-http:3000, jenkins:8080, nexus:8081). 그래서 UI 로 설치하면
// gitea·harbor·jenkins·nexus 라우트가 존재하지 않는 서비스를 가리켰고, 설치는
// 성공했는데 그 주소만 열리지 않았다(2026-08-21 운영에서 실측).
//
// 서버가 마법사의 매니페스트를 받아 이 표로 바로잡는다 — 두 곳이 갈라져도
// 실제로 적용되는 값은 하나다.
var gatewayBackends = map[string]GatewayBackend{
	"gitlab":     {Service: "gitlab-webservice-default", Port: 8181},
	"argocd":     {Service: "argo-cd-argocd-server", Port: 80},
	"argo-cd":    {Service: "argo-cd-argocd-server", Port: 80},
	"gitea":      {Service: GiteaHTTPServiceName, Port: GiteaServicePort},
	"jenkins":    {Service: JenkinsServiceName, Port: JenkinsServicePort},
	"harbor":     {Service: HarborServiceName, Port: HarborServicePort},
	"nexus":      {Service: NexusServiceName, Port: NexusServicePort},
	"minio":      {Service: MinIOConsoleServiceName, Port: MinIOConsoleServicePort},
	"grafana":    {Service: "grafana", Port: 80},
	"prometheus": {Service: "kube-prometheus-stack-prometheus", Port: 9090},
	"opensearch": {Service: "opensearch-cluster-master", Port: 9200},
	"openbao":    {Service: "openbao", Port: 8200},
}

// GatewayBackendForTool 은 도구 이름으로 백엔드를 찾는다.
//
// 이름 표기는 카탈로그·설정·매니페스트마다 흔들린다("Argo CD" / "argo-cd" /
// "argocd"). 정규화한 뒤에 본다.
func GatewayBackendForTool(tool string) (GatewayBackend, bool) {
	key := strings.ToLower(strings.TrimSpace(tool))
	key = strings.ReplaceAll(key, "_", "-")
	key = strings.Join(strings.Fields(key), "-")
	backend, ok := gatewayBackends[key]
	return backend, ok
}

// GatewayBackendForServiceAlias 는 마법사가 지어낸 이름을 실제 백엔드로 옮긴다.
//
// 마법사는 모르는 도구를 "<도구>-svc" 로 만든다. 그 이름에서 도구를 되짚어
// 올바른 서비스와 포트를 찾는다.
func GatewayBackendForServiceAlias(service string) (GatewayBackend, bool) {
	name := strings.ToLower(strings.TrimSpace(service))
	if backend, ok := GatewayBackendForTool(strings.TrimSuffix(name, "-svc")); ok {
		return backend, true
	}
	return GatewayBackend{}, false
}
