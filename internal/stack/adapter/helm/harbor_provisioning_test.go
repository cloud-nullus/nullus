package helm

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

func registryConfig(name string) domain.StackConfig {
	cfg := domain.StackConfig{}
	cfg.Artifacts.ContainerRegistry = domain.ToolSelection{Enabled: true, Name: name}
	return cfg
}

// Harbor 는 push 전에 프로젝트가 존재해야 한다.
//
// 없으면 첫 이미지 push 가 "unauthorized: project <name> not found" 로 죽는다 —
// 스택은 정상 설치됐고 빌드도 성공했는데 push 만 실패하는, 원인이 먼 실패다.
// 실제 파이프라인 검증에서 이렇게 막혔다.
func TestHarborProjectScript_CreatesProjectIdempotently(t *testing.T) {
	script := harborProjectScript("nullus")

	assert.Contains(t, script, "/api/v2.0/projects")
	assert.Contains(t, script, `\"project_name\":\"nullus\"`)
	// 재실행은 정상 경로다 — 스택 재배포마다 프로비저닝이 다시 돈다.
	assert.Contains(t, script, "409",
		"이미 있으면 성공으로 다뤄야 재배포가 실패하지 않는다")
}

// 프로젝트 이름은 이미지 저장소 접두사와 같아야 한다.
//
// CI/CD 모듈의 레지스트리 리졸버가 harbor.<domain>/<group>/<app> 를 만들므로,
// 여기서 만드는 프로젝트가 그 <group> 이 아니면 push 가 여전히 막힌다.
// 모듈 간 직접 import 가 금지돼 값을 조립 지점(main.go)에서 주입받는다.
func TestOrchestrator_ImageProjectName(t *testing.T) {
	o := NewOrchestrator(nil, []byte("k"), "nullus-gj3")
	assert.Equal(t, defaultImageProjectName, o.harborProjectName())

	o2 := NewOrchestrator(nil, []byte("k"), "nullus-gj3", WithImageProjectName("acme"))
	assert.Equal(t, "acme", o2.harborProjectName())
}

// Harbor 를 고른 스택에서만 프로비저닝이 돈다.
func TestProvisioningHarborStep_OnlyWhenHarborSelected(t *testing.T) {
	harbor := NewOrchestrator(nil, []byte("k"), "nullus-gj3")
	harbor.SetStackConfig(registryConfig("Harbor"))
	assert.True(t, harbor.IsStepEnabled("provisioning_harbor"))

	nexus := NewOrchestrator(nil, []byte("k"), "nullus-gj3")
	nexus.SetStackConfig(registryConfig("Nexus"))
	assert.False(t, nexus.IsStepEnabled("provisioning_harbor"))
}

// 이름이 JSON 을 깨뜨리면 조용히 다른 프로젝트를 만들거나 실패한다.
func TestHarborProjectScript_RejectsUnsafeProjectName(t *testing.T) {
	script := harborProjectScript(`ev"il`)
	assert.NotContains(t, script, `ev"il`)
	assert.True(t, strings.Contains(script, defaultImageProjectName),
		"안전하지 않은 이름은 기본값으로 떨어뜨린다")
}

// 접속 도메인이 TLS 로 서비스되면 도구 주소도 https 여야 한다.
//
// 예전에는 SSO 켜짐 여부로 스킴을 정했다. 스킴은 도메인이 TLS 로 열려 있느냐에
// 달린 것이지 SSO 와는 별개인데 그 둘을 묶어 놓아서, SSO 없이 TLS 만 켠 스택은
// http 를 받았다.
//
// 그 결과가 2026-08-21 운영에서 드러났다. Harbor 의 externalURL 이
// http://harbor.<도메인> 이 되고, Harbor 는 blob 업로드 주소(Location)를 그
// 스킴으로 돌려준다. 게이트웨이는 http 를 https 로 308 리다이렉트하는데, 20MB
// 본문이 실린 PATCH 에 308 이 오면 docker 는 연결을 끊는다 — push 가 모든
// layer 를 재시도하다 EOF 로 죽었다.
func TestToolURLScheme_HTTPSWhenAccessDomainUsesTLS(t *testing.T) {
	o := &Orchestrator{}
	o.SetStackConfig(domain.StackConfig{
		AccessDomain:    "nullus.io",
		AccessDomainTLS: &domain.AccessDomainTLSConfig{Enabled: true},
	})

	assert.Equal(t, "https", o.toolURLScheme(),
		"TLS 로 열린 도메인에 http 를 돌려주면 업로드가 308 에서 끊긴다")
}

func TestToolURLScheme_HTTPWhenNeitherTLSNorSSO(t *testing.T) {
	o := &Orchestrator{}
	o.SetStackConfig(domain.StackConfig{AccessDomain: "nullus.local"})

	assert.Equal(t, "http", o.toolURLScheme())
}

// TLS 를 켠 스택의 Harbor externalURL 은 https 여야 한다. 이 값이 곧 업로드
// Location 의 스킴이 된다.
func TestHarborExternalURL_UsesHTTPSWhenTLSEnabled(t *testing.T) {
	o := &Orchestrator{}
	cfg := domain.StackConfig{
		AccessDomain:    "nullus.io",
		AccessDomainTLS: &domain.AccessDomainTLSConfig{Enabled: true},
	}
	o.SetStackConfig(cfg)

	values := o.harborExternalURLValues(&cfg)

	assert.Equal(t, "https://harbor.nullus.io", values["externalURL"])
}
