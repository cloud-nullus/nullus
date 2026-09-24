package helm

import (
	"fmt"
	"strings"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// runnerHostAlias 는 CI 잡 파드의 /etc/hosts 에 들어갈 한 줄이다.
type runnerHostAlias struct {
	IP        string
	Hostnames []string
}

// gitLabRunnerConfigTOML 은 러너의 config.template.toml 을 만든다.
//
// 차트 0.72.0 은 runners.privileged 를 읽지 않는다 — 설정은 runners.config 의
// TOML 로 넣어야 config.toml 에 반영된다. privileged 없이는 docker:dind 서비스가
// 기동하지 못해 파이프라인의 이미지 빌드 단계가 통째로 실패한다.
//
// helperImage 가 비면 그 줄을 넣지 않고 러너가 고르는 이미지를 그대로 둔다.
// 기본값과 보정이 각자 TOML 을 들고 있으면 한쪽만 고쳐져 갈라지므로, 두 경로가
// 이 함수 하나를 쓴다.
func gitLabRunnerConfigTOML(helperImage string, aliases []runnerHostAlias) string {
	config := `[[runners]]
  [runners.kubernetes]
    namespace = "{{.Release.Namespace}}"
    image = "alpine"
    privileged = true
`
	if strings.TrimSpace(helperImage) != "" {
		config += fmt.Sprintf("    helper_image = %q\n", helperImage)
	}
	for _, alias := range aliases {
		if strings.TrimSpace(alias.IP) == "" || len(alias.Hostnames) == 0 {
			continue
		}
		quoted := make([]string, 0, len(alias.Hostnames))
		for _, h := range alias.Hostnames {
			quoted = append(quoted, fmt.Sprintf("%q", h))
		}
		config += fmt.Sprintf(`    [[runners.kubernetes.host_aliases]]
      ip = %q
      hostnames = [%s]
`, alias.IP, strings.Join(quoted, ", "))
	}
	return config
}

// gitLabRunnerArchValues 는 노드 아키텍처에 맞는 helper 이미지를 실은 러너 values 다.
// 고를 수 없으면 nil 이고, 그러면 기본값이 그대로 남는다.
func gitLabRunnerArchValues(nodeArchs []string) map[string]any {
	image, _ := domain.GitLabRunnerHelperImage(nodeArchs, domain.GitLabRunnerAppVersion)
	if image == "" {
		return nil
	}
	return map[string]any{
		"runners": map[string]any{
			"config": gitLabRunnerConfigTOML(image, nil),
		},
	}
}

// runnerCIHostnames 는 CI 잡이 실제로 이름으로 닿아야 하는 스택 도구다.
//
// 잡은 스택 자신의 GitLab 에서 소스를 받고(clone 주소는 GitLab 의 외부 주소다)
// 만든 이미지를 스택 레지스트리에 올린다. 둘 다 접속 도메인 이름이고, 그 이름은
// 클러스터 안에서는 아무도 풀어 주지 않는다 — 게이트웨이가 Host 헤더로 라우팅하므로
// 서비스 주소로 대신할 수도 없다(포트가 다르다).
func runnerCIHostnames(accessDomain string) []string {
	domainName := strings.TrimSpace(accessDomain)
	if domainName == "" {
		return nil
	}
	return []string{
		"gitlab." + domainName,
		"registry." + domainName,
	}
}

// gitLabRunnerHostAliasValues 는 CI 잡 파드가 스택 도구 이름을 풀게 하는 러너 values 다.
//
// 네임스페이스 안에서 끝낸다. 클러스터 DNS(CoreDNS)를 고치면 남의 네임스페이스까지
// 영향을 받고, 개발자 머신의 /etc/hosts 는 클러스터 안에서 아무 소용이 없다.
func gitLabRunnerHostAliasValues(gatewayIP, accessDomain, helperImage string) map[string]any {
	ip := strings.TrimSpace(gatewayIP)
	hostnames := runnerCIHostnames(accessDomain)
	if ip == "" || len(hostnames) == 0 {
		return nil
	}
	return map[string]any{
		"runners": map[string]any{
			"config": gitLabRunnerConfigTOML(helperImage, []runnerHostAlias{{IP: ip, Hostnames: hostnames}}),
		},
	}
}

// gitLabRunnerValues 는 이 클러스터·이 스택에 맞춘 러너 values 다.
//
// helper 이미지와 host_aliases 는 같은 키(runners.config)에 들어가므로 한 번에
// 만든다. 따로 병합하면 뒤에 오는 쪽이 앞의 것을 통째로 지운다.
func (o *Orchestrator) gitLabRunnerValues() map[string]any {
	helperImage, _ := domain.GitLabRunnerHelperImage(o.nodeArchitectures(), domain.GitLabRunnerAppVersion)

	o.mu.Lock()
	cfg := o.stackConfig
	o.mu.Unlock()

	var accessDomain string
	if cfg != nil {
		accessDomain = cfg.AccessDomain
	}
	if aliases := gitLabRunnerHostAliasValues(o.gatewayClusterIP(), accessDomain, helperImage); aliases != nil {
		return aliases
	}
	if helperImage == "" {
		return nil
	}
	return map[string]any{
		"runners": map[string]any{
			"config": gitLabRunnerConfigTOML(helperImage, nil),
		},
	}
}
