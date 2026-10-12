package domain

import (
	"regexp"
	"strings"
)

// 설치는 스택 게이트웨이의 ClusterIP 를 러너(CI 잡 host_aliases)와 Argo CD(global.hostAliases)에
// 박는다. 스택 도구 이름(gitlab.<도메인> 등)은 클러스터 안에서 아무도 풀어 주지 않기 때문이다.
// 그 주소가 바뀌면(다른 클러스터로 복구, 게이트웨이 Service 재생성) 두 곳 모두 옛 주소를 가리켜
// CI 는 clone 에서, Argo CD 는 매니페스트 조회에서 "이름을 풀 수 없다" 로 멈춘다.
//
// 여기 함수들은 helm values 안의 그 주소만 새 주소로 바꾼다. 다른 항목(사용자가 더한 호스트,
// helper 이미지, CA 볼륨)은 건드리지 않는다.

// GatewayPinnedHostnames 는 게이트웨이 주소를 박아 두는 스택 도구 이름이다. CI 잡은 스택 GitLab 에서
// 소스를 받고 스택 레지스트리에 이미지를 올린다. 접근 도메인이 없으면 박을 것도 없다.
func GatewayPinnedHostnames(accessDomain string) []string {
	domainName := strings.TrimSpace(accessDomain)
	if domainName == "" {
		return nil
	}
	return []string{"gitlab." + domainName, "registry." + domainName}
}

// RepinArgoCDHostAliases 는 Argo CD values 의 global.hostAliases 중 스택 도구 항목의 ip 를 바꾼다.
// 바꿨는지와 바뀌기 전 주소들을 돌려준다. values 를 그 자리에서 고친다.
func RepinArgoCDHostAliases(values map[string]any, hostnames []string, ip string) (bool, []string) {
	ip = strings.TrimSpace(ip)
	if ip == "" || len(hostnames) == 0 {
		return false, nil
	}
	global, _ := values["global"].(map[string]any)
	aliases, _ := global["hostAliases"].([]any)
	var old []string
	for _, a := range aliases {
		entry, ok := a.(map[string]any)
		if !ok || !anyPinned(stringsOf(entry["hostnames"]), hostnames) {
			continue
		}
		cur, _ := entry["ip"].(string)
		if cur == ip {
			continue
		}
		entry["ip"] = ip
		old = append(old, cur)
	}
	return len(old) > 0, old
}

// runnerHostAliasBlock 은 설치가 쓰는 러너 TOML 의 host_aliases 블록이다(ip 다음 hostnames).
var runnerHostAliasBlock = regexp.MustCompile(
	`(\[\[runners\.kubernetes\.host_aliases\]\]\s*\n\s*ip\s*=\s*)"([^"]*)"(\s*\n\s*hostnames\s*=\s*\[([^\]]*)\])`)

// RepinRunnerHostAliases 는 러너 values 의 runners.config(TOML) 중 스택 도구 host_aliases 블록의 ip 를
// 바꾼다. 바꿨는지와 바뀌기 전 주소들을 돌려준다. values 를 그 자리에서 고친다.
func RepinRunnerHostAliases(values map[string]any, hostnames []string, ip string) (bool, []string) {
	ip = strings.TrimSpace(ip)
	if ip == "" || len(hostnames) == 0 {
		return false, nil
	}
	runners, _ := values["runners"].(map[string]any)
	config, _ := runners["config"].(string)
	if config == "" {
		return false, nil
	}
	var old []string
	updated := runnerHostAliasBlock.ReplaceAllStringFunc(config, func(block string) string {
		m := runnerHostAliasBlock.FindStringSubmatch(block)
		if m[2] == ip || !anyPinned(tomlStrings(m[4]), hostnames) {
			return block
		}
		old = append(old, m[2])
		return m[1] + `"` + ip + `"` + m[3]
	})
	if len(old) == 0 {
		return false, nil
	}
	runners["config"] = updated
	return true, old
}

func anyPinned(have, pinned []string) bool {
	for _, h := range have {
		for _, p := range pinned {
			if strings.EqualFold(strings.TrimSpace(h), p) {
				return true
			}
		}
	}
	return false
}

func stringsOf(v any) []string {
	switch x := v.(type) {
	case []string:
		return x
	case []any:
		out := make([]string, 0, len(x))
		for _, e := range x {
			if s, ok := e.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// tomlStrings 는 `"a", "b"` 꼴의 TOML 문자열 배열 본문을 읽는다.
func tomlStrings(body string) []string {
	var out []string
	for _, part := range strings.Split(body, ",") {
		if s := strings.Trim(strings.TrimSpace(part), `"`); s != "" {
			out = append(out, s)
		}
	}
	return out
}
