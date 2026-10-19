package imagescan

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// 노출 판정 — 같은 CVE 라도 클러스터 밖에서 닿는 서비스의 것을 먼저 고친다(설치 OSS CVE 대응
// 정책). 스택은 Gateway API(HTTPRoute)로 도구를 연다. HTTPRoute 의 Service 백엔드를 따라가
// Service selector 에 맞는 파드를 "노출" 로 본다. Ingress · NodePort · LoadBalancer 는 보지
// 않는다 — 스택이 만들지 않는 경로다.

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
			Name string `json:"name"`
		} `json:"metadata"`
		Spec struct {
			Selector map[string]string `json:"selector"`
		} `json:"spec"`
	} `json:"items"`
}

// exposedPods 는 네임스페이스의 HTTPRoute 로 밖에 열린 파드와 그 호스트 이름이다.
// 경로 목록이 비어 있으면 노출된 파드가 없다.
func exposedPods(namespace string, podsRaw, routesRaw, servicesRaw []byte) (map[string][]string, error) {
	if len(strings.TrimSpace(string(routesRaw))) == 0 {
		return nil, nil
	}
	var routes routeList
	if err := json.Unmarshal(routesRaw, &routes); err != nil {
		return nil, fmt.Errorf("HTTPRoute 목록을 해석하지 못했습니다: %w", err)
	}
	// Service 이름 → 그 Service 로 들어오는 호스트 이름
	hostsByService := map[string][]string{}
	for _, r := range routes.Items {
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
	if len(hostsByService) == 0 {
		return nil, nil
	}

	var services serviceList
	if len(strings.TrimSpace(string(servicesRaw))) > 0 {
		if err := json.Unmarshal(servicesRaw, &services); err != nil {
			return nil, fmt.Errorf("Service 목록을 해석하지 못했습니다: %w", err)
		}
	}
	var pods podList
	if err := json.Unmarshal(podsRaw, &pods); err != nil {
		return nil, fmt.Errorf("파드 목록을 해석하지 못했습니다: %w", err)
	}

	exposed := map[string][]string{}
	for _, svc := range services.Items {
		hosts, routed := hostsByService[svc.Metadata.Name]
		if !routed || len(svc.Spec.Selector) == 0 {
			continue
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
