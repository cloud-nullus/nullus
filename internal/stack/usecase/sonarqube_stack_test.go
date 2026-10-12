package usecase

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// SonarQube 의 관리자 비밀번호는 프로비저닝이 Secret 값으로 바꿔 둔다. 연결정보가 그
// Secret 을 가리켜야 사용자가 첫 로그인을 할 수 있다.
func TestGetConnectionInfo_ListsSonarQubeAdmin(t *testing.T) {
	cfg := fullConfig()
	cfg.Security.SAST = domain.ToolSelection{Name: "SonarQube", Enabled: true}

	out, err := newConnUC(connStack(cfg)).Execute(context.Background(), "stk_1")
	require.NoError(t, err)

	var found *domain.ToolCredential
	for _, tool := range out.Tools {
		if tool.Name == "SonarQube" {
			tool := tool
			found = &tool
		}
	}
	require.NotNil(t, found, "SonarQube 관리자 계정이 연결정보에 없다")
	assert.Equal(t, domain.SonarQubeAdminUser, found.Username)
	assert.Equal(t, domain.SonarQubeSecret, found.SecretRef)
	assert.Equal(t, domain.SonarQubeAdminPasswordKey, found.SecretKey)
}

// 외부 SonarQube 는 스택이 세우지 않았으므로 스택의 Secret 을 안내하면 안 된다.
func TestGetConnectionInfo_SkipsExternalSonarQube(t *testing.T) {
	cfg := fullConfig()
	cfg.Security.SAST = domain.ToolSelection{Name: "SonarQube", Version: "external", Enabled: true}

	out, err := newConnUC(connStack(cfg)).Execute(context.Background(), "stk_1")
	require.NoError(t, err)

	for _, tool := range out.Tools {
		assert.NotEqual(t, "SonarQube", tool.Name)
	}
}

// ESO 가 만든 Secret 은 Helm 소유 표시가 없다. 네임스페이스를 통째로 회수하지 않는
// 자리(사용자가 고른 이름)에서는 이름으로 지워야 남지 않는다.
func TestDeleteStack_RemovesSonarQubeCredentialsByName(t *testing.T) {
	_, ok := legacyReleaseArtifactExactNames[domain.SonarQubeSecret]
	assert.True(t, ok)
}
