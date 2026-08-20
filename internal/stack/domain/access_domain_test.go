package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateAccessDomain_AcceptsRealDomains(t *testing.T) {
	for _, d := range []string{"nullus.io", "stack.nullus.io", "nullus-devsecops-stack.internal", "a.co"} {
		assert.NoError(t, ValidateAccessDomain(d), "domain %q", d)
	}
}

// 2026-08-20 운영 스택의 access_domain 이 ".io" 로 저장돼 있었다. 그 값으로
// HTTPRoute 호스트는 "jenkins..io", 인증서는 commonName ".io" / dnsNames "*..io"
// 가 만들어졌다 — 라우팅도 발급도 될 수 없는 매니페스트다. 아무도 막지 않았다.
func TestValidateAccessDomain_RejectsMalformedValues(t *testing.T) {
	for _, d := range []string{
		".io",
		"nullus..io",
		"nullus.io.",
		"-nullus.io",
		"nullus.io-",
		"nullus",
		"nullus io",
		"http://nullus.io",
		"*.nullus.io",
	} {
		assert.Error(t, ValidateAccessDomain(d), "domain %q 는 거부돼야 한다", d)
	}
}

func TestValidateAccessDomain_RejectsEmpty(t *testing.T) {
	err := ValidateAccessDomain("   ")
	require.Error(t, err)
}
