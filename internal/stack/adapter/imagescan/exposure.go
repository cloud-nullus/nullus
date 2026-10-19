package imagescan

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// 노출 판정 — 같은 CVE 라도 클러스터 밖에서 닿는 서비스의 것을 먼저 고친다(설치 OSS CVE 대응
// 정책). 두 길을 본다.
//   - 스택은 Gateway API(HTTPRoute)로 도구를 연다. HTTPRoute 의 Service 백엔드를 따라가 Service
//     selector 에 맞는 파드를 그 라우트의 호스트로 "노출" 로 본다.
//   - LoadBalancer · NodePort Service 가 고르는 파드도 노출이다. 스택 게이트웨이의 Envoy 데이터플레인이
//     그렇게 열려 밖의 요청을 직접 받으므로(브리지 Ingress 도 그것을 가리킨다) 네임스페이스의 모든
//     라우트 호스트로 노출된 것이다. 그 밖의 것은 호스트를 모른다.
// Ingress 는 보지 않는다 — 스택이 만드는 브리지 Ingress 는 Envoy 를 가리킬 뿐이다.

// gatewayNamespaceLabel 은 Envoy Gateway 가 데이터플레인 Service 에 붙이는 라벨이다.
const gatewayNamespaceLabel = "gateway.envoyproxy.io/owning-gateway-namespace"

type routeList struct {
	Items []struct {
		Spec struct {
			Hostnames []string `json:"hostnames"`
			Rules     []struct {
				BackendRefs []struct {
					Group     string `json:"group"`
					Kind      string `json:"kind"`
					Name      string `json:"name"`
					Namespace string `json:"namespace"`
				} `json:"backendRefs"`
			} `json:"rules"`
		} `json:"spec"`
	} `json:"items"`
}

type serviceList struct {
	Items []struct {
		Metadata struct {
			Name   string            `json:"name"`
			Labels map[string]string `json:"labels"`
		} `json:"metadata"`
		Spec struct {
			Type     string            `json:"type"`
			Selector map[string]string `json:"selector"`
		} `json:"spec"`
	} `json:"items"`
}

// exposedPods 는 네임스페이스에서 밖에 열린 파드와 그 호스트 이름이다(호스트를 모르면 빈 목록).
// 라우트 목록은 비어 있어도 된다 — Gateway API CRD 가 없는 클러스터다.
func exposedPods(namespace string, podsRaw, routesRaw, servicesRaw []byte) (map[string][]string, error) {
	var routes routeList
	if len(strings.TrimSpace(string(routesRaw))) > 0 {
		if err := json.Unmarshal(routesRaw, &routes); err != nil {
			return nil, fmt.Errorf("HTTPRoute 목록을 해석하지 못했습니다: %w", err)
		}
	}
	// Service 이름 → 그 Service 로 들어오는 호스트 이름, 그리고 네임스페이스의 모든 라우트 호스트
	hostsByService := map[string][]string{}
	allHosts := []string{}
	for _, r := range routes.Items {
		for _, h := range r.Spec.Hostnames {
			allHosts = appendUnique(allHosts, h)
		}
		for _, rule := range r.Spec.Rules {
			for _, ref := range rule.BackendRefs {
				if !isServiceRef(ref.Group, ref.Kind) || (ref.Namespace != "" && ref.Namespace != namespace) {
					continue
				}
				hosts := hostsByService[ref.Name]
				if hosts == nil {
					hosts = []string{}
				}
				for _, h := range r.Spec.Hostnames {
					hosts = appendUnique(hosts, h)
				}
				hostsByService[ref.Name] = hosts
			}
		}
	}

	var services serviceList
	if len(strings.TrimSpace(string(servicesRaw))) > 0 {
		if err := json.Unmarshal(servicesRaw, &services); err != nil {
			return nil, fmt.Errorf("서비스 목록을 해석하지 못했습니다: %w", err)
		}
	}
	var pods podList
	if err := json.Unmarshal(podsRaw, &pods); err != nil {
		return nil, fmt.Errorf("파드 목록을 해석하지 못했습니다: %w", err)
	}

	exposed := map[string][]string{}
	for _, svc := range services.Items {
		if len(svc.Spec.Selector) == 0 {
			continue
		}
		hosts, routed := hostsByService[svc.Metadata.Name]
		if !routed {
			if !opensOutside(svc.Spec.Type) {
				continue
			}
			hosts = []string{}
			if svc.Metadata.Labels[gatewayNamespaceLabel] == namespace {
				hosts = allHosts
			}
		}
		for _, p := range pods.Items {
			if !matchesSelector(p.Metadata.Labels, svc.Spec.Selector) {
				continue
			}
			list := exposed[p.Metadata.Name]
			if list == nil {
				list = []string{}
			}
			for _, h := range hosts {
				list = appendUnique(list, h)
			}
			sort.Strings(list)
			exposed[p.Metadata.Name] = list
		}
	}
	return exposed, nil
}

// opensOutside 는 Service 종류가 노드 밖에서 닿는 것인지다.
func opensOutside(serviceType string) bool {
	return serviceType == "LoadBalancer" || serviceType == "NodePort"
}

// isServiceRef 는 백엔드가 core Service 인지다(Gateway API 기본값: group "" · kind Service).
func isServiceRef(group, kind string) bool {
	return (group == "" || group == "core") && (kind == "" || kind == "Service")
}

func matchesSelector(labels, selector map[string]string) bool {
	for k, v := range selector {
		if labels[k] != v {
			return false
		}
	}
	return true
}

func appendUnique(list []string, v string) []string {
	for _, x := range list {
		if x == v {
			return list
		}
	}
	return append(list, v)
}
