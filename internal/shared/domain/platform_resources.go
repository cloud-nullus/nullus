package domain

import (
	"fmt"
	"strings"
)

// 플랫폼이 설치하는 공유 인프라의 Helm 릴리스 이름.
//
// 여기 있는 이유는 두 모듈이 같은 이름을 봐야 하기 때문이다 — stack 은 이
// 이름으로 차트를 설치하고 접속 정보를 안내하고, admin 의 시크릿 회전은 같은
// 이름으로 워크로드를 재시작한다. 모듈끼리 서로의 internal 을 참조할 수 없으므로
// 이름의 단일 출처를 shared 에 둔다.
//
// Helm 은 릴리스명을 그대로 Service 이름에 쓰므로, 이 값이 곧 클러스터 안의
// 접속 주소이기도 하다. 한쪽만 바꾸면 다른 쪽이 조용히 어긋난다.
const (
	// PostgresReleaseName 은 플랫폼이 설치하는 PostgreSQL 릴리스다.
	PostgresReleaseName = "nullus-postgresql"
	// MinIOReleaseName 은 플랫폼이 설치하는 MinIO 릴리스다.
	MinIOReleaseName = "nullus-minio"
)

// OpenTelemetry Collector 의 이름과 주소 규칙.
//
// stack 은 이 이름으로 차트를 설치하고, cicd 는 배포하는 앱에 "어디로 텔레메트리를
// 보낼지" 를 넣어 준다. 두 모듈이 같은 주소를 봐야 하므로 규칙을 여기 둔다 —
// 한쪽이 문자열을 흉내 내면 차트 이름이 바뀔 때 조용히 어긋난다.
const (
	// OTelCollectorReleaseName 은 게이트웨이 수집기의 릴리스명이다.
	//
	// 차트 이름(opentelemetry-collector)을 그대로 쓰지 않는 이유는 추적 계층
	// 단계가 같은 차트를 설치할 수 있어 릴리스명이 충돌하기 때문이다.
	OTelCollectorReleaseName = "otel-collector"
	// OTelAgentReleaseName 은 노드마다 서는 로그 수집 에이전트의 릴리스명이다.
	OTelAgentReleaseName = "otel-agent"

	// OTLP 수신 포트. 차트 기본값이며 애플리케이션이 붙는 지점이다.
	OTelCollectorOTLPGRPCPort = 4317
	OTelCollectorOTLPHTTPPort = 4318

	// TrivyReleaseName 은 이미지 취약점 스캐너의 릴리스명이다.
	//
	// stack 은 이 이름으로 차트를 설치하고, cicd 는 CI 잡이 붙을 주소를 만든다.
	// OTLP 와 같은 이유로 규칙을 여기 둔다.
	TrivyReleaseName = "trivy"
	// TrivyServicePort 는 client 가 CVE 매칭을 요청하는 포트다.
	// aqua/trivy 차트의 service.port 기본값과 같아야 한다.
	TrivyServicePort = 4954
)

// TrivyClientImage 는 스택 서버에 붙는 trivy client 이미지다. 서버(스택 차트)와
// 버전을 맞춘다 — client 와 server 의 버전이 갈리면 스캔 요청이 거절될 수 있다.
//
// RuntimeImages 에는 넣지 않는다. 이 이미지를 쓰는 설치 이미지 스캔 Job 은
// 에어갭에서 돌지 않는다.
const TrivyClientImage = "aquasec/trivy:0.74.0"

// TrivyServerEndpoint 는 CI 잡의 trivy client 가 붙을 주소다.
//
// 스캐너를 고르지 않은 스택에서는 네임스페이스만으로 주소를 만들면 안 된다 —
// 호출부가 "스캐너 있음" 으로 오해한다. 네임스페이스가 비면 빈 문자열이다.
func TrivyServerEndpoint(namespace string) string {
	ns := strings.TrimSpace(namespace)
	if ns == "" {
		return ""
	}
	return fmt.Sprintf("http://%s.%s.svc.cluster.local:%d",
		TrivyReleaseName, ns, TrivyServicePort)
}

// OTelCollectorServiceName 은 수집기 Service 이름이다.
// 차트의 fullname 규칙이 "<릴리스>-<차트>" 라 릴리스명만으로는 맞지 않는다.
func OTelCollectorServiceName() string {
	return OTelCollectorReleaseName + "-opentelemetry-collector"
}

// OTelCollectorOTLPGRPCEndpoint 는 애플리케이션이 OTLP/gRPC 로 보낼 주소다.
func OTelCollectorOTLPGRPCEndpoint(namespace string) string {
	return otelCollectorEndpoint(namespace, OTelCollectorOTLPGRPCPort)
}

// OTelCollectorOTLPHTTPEndpoint 는 애플리케이션이 OTLP/HTTP 로 보낼 주소다.
func OTelCollectorOTLPHTTPEndpoint(namespace string) string {
	return otelCollectorEndpoint(namespace, OTelCollectorOTLPHTTPPort)
}

func otelCollectorEndpoint(namespace string, port int) string {
	ns := strings.TrimSpace(namespace)
	if ns == "" {
		ns = DefaultStackNamespace
	}
	return fmt.Sprintf("%s.%s.svc.cluster.local:%d", OTelCollectorServiceName(), ns, port)
}

// DefaultStackNamespace 는 스택 네임스페이스가 정해지지 않았을 때의 기본값이다.
const DefaultStackNamespace = "nullus"
