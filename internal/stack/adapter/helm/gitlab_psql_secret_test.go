package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// GitLab 은 DB 비밀번호를 K8s Secret 에서 읽는다. 그런데 PostgreSQL 차트는
// auth.existingSecret=nullus-postgresql-credentials 로 설치되므로 bitnami 차트가
// 자기 이름(nullus-postgresql)으로 Secret 을 만들지 않는다. GitLab 이 그 이름을
// 가리키면 파드가
//
//	MountVolume.SetUp failed ... secret "nullus-postgresql" not found
//
// 으로 기동하지 못한다. 두 참조는 반드시 같은 이름이어야 한다.
func psqlPasswordSecret(t *testing.T, values map[string]any) map[string]any {
	t.Helper()

	global, ok := values["global"].(map[string]any)
	require.True(t, ok, "global 섹션이 있어야 한다")
	psql, ok := global["psql"].(map[string]any)
	require.True(t, ok, "global.psql 섹션이 있어야 한다")
	password, ok := psql["password"].(map[string]any)
	require.True(t, ok, "global.psql.password 섹션이 있어야 한다")

	return password
}

func TestDefaultValues_GitLabPsqlUsesProvisionedSecret(t *testing.T) {
	password := psqlPasswordSecret(t, DefaultValues("installing_gitlab"))

	assert.Equal(t, ProvisionedPostgresSecret, password["secret"])
	assert.Equal(t, "password", password["key"])
}

func TestGitLabExternalSharedServiceValues_UsesProvisionedSecret(t *testing.T) {
	o := NewOrchestrator(nil, nil, "devsecops")

	password := psqlPasswordSecret(t, o.gitlabExternalSharedServiceValues(nil))

	assert.Equal(t, ProvisionedPostgresSecret, password["secret"])
	assert.Equal(t, "password", password["key"])
}

// PostgreSQL 차트가 참조하는 existingSecret 과 GitLab 이 읽는 Secret 이름이
// 같은 값을 가리키는지 못박는다.
func TestPostgresExistingSecretMatchesGitLabReference(t *testing.T) {
	o := NewOrchestrator(nil, nil, "devsecops")

	pgValues := o.sharedPostgresValues(nil)
	auth, ok := pgValues["auth"].(map[string]any)
	require.True(t, ok)

	password := psqlPasswordSecret(t, o.gitlabExternalSharedServiceValues(nil))

	assert.Equal(t, auth["existingSecret"], password["secret"],
		"GitLab 이 읽는 Secret 은 PostgreSQL 차트의 existingSecret 과 같아야 한다")
}
