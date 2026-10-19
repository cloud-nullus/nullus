package domain

import (
	"fmt"
	"sort"
	"strings"
)

// 설치 OSS CVE 고지 — 스택이 설치한 이미지에서 알릴 취약점을 고른다.
//
// 알릴 것은 "수정판이 있는 CRITICAL" 중 지난 스캔에 없던 것이다. 수정판이 없으면 지금 할 수
// 있는 일이 없고, HIGH 는 화면과 정기 점검으로 본다. 같은 내용을 매 스캔마다 보내면 아무도
// 읽지 않게 된다(docs/11_기능설계/Nullus_설치_OSS_CVE_대응_정책.md).

const severityCritical = "CRITICAL"

// CVEFinding 은 알릴 취약점 하나다. 여러 이미지에 같은 CVE 가 있으면 하나로 묶는다.
type CVEFinding struct {
	ID string
	// Packages 는 "패키지 설치버전 → 수정버전" 이다.
	Packages []string
	Images   []string
	// Exposed 는 그 CVE 가 든 이미지 중 하나라도 클러스터 밖에 열려 있는지다.
	Exposed bool
	Hosts   []string
}

// NewFixableCriticals 는 known(지난 스캔에서 이미 본 것)에 없는 "수정판 있는 CRITICAL" 을 고른다.
// 외부에 노출된 것을 앞에, 같으면 ID 순으로 둔다. 목록이 기록되지 않은 이미지는 볼 수 없다.
func NewFixableCriticals(known map[string]bool, scans []StackImageScan) []CVEFinding {
	byID := map[string]*CVEFinding{}
	for _, scan := range scans {
		if !scan.VulnerabilitiesRecorded {
			continue
		}
		for _, v := range scan.Vulnerabilities {
			if !strings.EqualFold(v.Severity, severityCritical) || strings.TrimSpace(v.FixedVersion) == "" || known[v.ID] {
				continue
			}
			f, ok := byID[v.ID]
			if !ok {
				f = &CVEFinding{ID: v.ID}
				byID[v.ID] = f
			}
			f.Packages = appendUnique(f.Packages, fmt.Sprintf("%s %s → %s", v.PkgName, v.InstalledVersion, v.FixedVersion))
			f.Images = appendUnique(f.Images, scan.Image)
			if scan.Exposed {
				f.Exposed = true
				for _, h := range scan.ExposedHosts {
					f.Hosts = appendUnique(f.Hosts, h)
				}
			}
		}
	}

	out := make([]CVEFinding, 0, len(byID))
	for _, f := range byID {
		sort.Strings(f.Packages)
		sort.Strings(f.Images)
		sort.Strings(f.Hosts)
		out = append(out, *f)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Exposed != out[j].Exposed {
			return out[i].Exposed
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func appendUnique(list []string, v string) []string {
	for _, x := range list {
		if x == v {
			return list
		}
	}
	return append(list, v)
}
