package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"

	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
)

func vuln(id, severity, pkg, installed, fixed string) shareddomain.ImageVulnerability {
	return shareddomain.ImageVulnerability{ID: id, Severity: severity, PkgName: pkg, InstalledVersion: installed, FixedVersion: fixed}
}

// 알릴 것은 "수정판이 있는 CRITICAL" 중 지난 스캔에 없던 것이다. 수정판이 없으면 할 수 있는
// 일이 없고, HIGH 는 화면과 정기 점검으로 본다(정책 문서 — 설치 OSS CVE 대응).
func TestNewFixableCriticals(t *testing.T) {
	scans := []StackImageScan{
		{
			Image: "gitlab-webservice:17.7", ImageDigest: "sha256:web", Status: ImageScanStatusScanned,
			VulnerabilitiesRecorded: true, Exposed: true, ExposedHosts: []string{"gitlab.nullus.local"},
			Vulnerabilities: []shareddomain.ImageVulnerability{
				vuln("CVE-NEW", "CRITICAL", "openssl", "3.0.1", "3.0.9"),
				vuln("CVE-OLD", "CRITICAL", "libc", "2.35", "2.36"),
				vuln("CVE-NOFIX", "CRITICAL", "zlib", "1.2", ""),
				vuln("CVE-HIGH", "HIGH", "curl", "8.0", "8.1"),
			},
		},
		{
			Image: "gitlab-sidekiq:17.7", ImageDigest: "sha256:sk", Status: ImageScanStatusScanned,
			VulnerabilitiesRecorded: true,
			Vulnerabilities: []shareddomain.ImageVulnerability{
				vuln("CVE-NEW", "CRITICAL", "openssl", "3.0.1", "3.0.9"),
				vuln("CVE-INTERNAL", "CRITICAL", "glibc", "2.35", "2.36"),
			},
		},
		// 목록이 없는 이미지(스캔 실패)는 볼 수 없다.
		{Image: "broken:1", ImageDigest: "sha256:b", Status: ImageScanStatusFailed},
	}

	got := NewFixableCriticals(map[string]bool{"CVE-OLD": true}, scans)

	assert.Equal(t, []CVEFinding{
		{
			ID: "CVE-NEW", Packages: []string{"openssl 3.0.1 → 3.0.9"},
			Images:  []string{"gitlab-sidekiq:17.7", "gitlab-webservice:17.7"},
			Exposed: true, Hosts: []string{"gitlab.nullus.local"},
		},
		{
			ID: "CVE-INTERNAL", Packages: []string{"glibc 2.35 → 2.36"},
			Images: []string{"gitlab-sidekiq:17.7"},
		},
	}, got, "외부에 노출된 이미지의 것이 앞이다")
}

func TestNewFixableCriticals_NothingNew(t *testing.T) {
	scans := []StackImageScan{{
		Image: "a:1", ImageDigest: "sha256:a", Status: ImageScanStatusScanned, VulnerabilitiesRecorded: true,
		Vulnerabilities: []shareddomain.ImageVulnerability{vuln("CVE-1", "CRITICAL", "p", "1", "2")},
	}}
	assert.Empty(t, NewFixableCriticals(map[string]bool{"CVE-1": true}, scans))
}
