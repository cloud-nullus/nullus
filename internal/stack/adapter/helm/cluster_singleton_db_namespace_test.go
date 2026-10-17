package helm

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/adapter/repository"
	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// 클러스터 공용 애드온의 네임스페이스 핀은 DB 메타데이터가 비어 있어도 지켜져야 한다.
//
// chartSpecForStep 은 helmStepMetadataRepo 를 코드 기본값보다 먼저 본다. 그래서
// DB 행의 namespace 가 비면 helm_step_metadata.go 의 핀이 그대로 무시되고, 애드온이
// 스택 네임스페이스로 들어간다. 기존 TestMetricsServerInstallsIntoKubeSystem 은
// defaultChartSpecForStep 을 직접 보기 때문에 이 경로를 잡지 못했다.
//
// metrics-server 가 스택 네임스페이스로 들어가면 APIService v1beta1.metrics.k8s.io 가
// 그 네임스페이스의 Service 를 가리킨다. 스택을 지우면 Service 만 사라지고 APIService 는
// 죽은 대상을 계속 가리켜 API discovery 가 실패하고, 그러면 그 클러스터의 **모든**
// 네임스페이스 삭제가 교착된다(stale GroupVersion discovery).
//
// 2026-10-17 실측: 코드에 핀이 있는 커밋으로 설치했는데도
//
//	v1beta1.metrics.k8s.io -> metrics-server.nullus-gitlab-argocd-sonarqube.svc
//
// 였다. DB 시드(000056)의 해당 행 namespace 가 NULL 이었기 때문이다.
func TestSharedAddonNamespaceSurvivesEmptyDBMetadata(t *testing.T) {
	for _, step := range []string{stepInstallingCertManager, "installing_metrics_server"} {
		t.Run(step, func(t *testing.T) {
			want, ok := defaultChartSpecForStep(step)
			require.True(t, ok)
			require.NotEmpty(t, want.Namespace, "이 테스트의 전제")

			repo := repository.NewMemoryHelmStepMetadataRepository()
			// DB 행은 있지만 namespace 가 비어 있는 상태 — 000056 시드의 metrics-server 가 그랬다.
			require.NoError(t, repo.Create(context.Background(), &domain.HelmStepMetadata{
				StepName:  step,
				ChartName: want.ChartName,
				RepoURL:   want.RepoURL,
				Version:   want.Version,
				Namespace: "",
			}))

			o := &Orchestrator{namespace: "nullus-some-stack", helmStepMetadataRepo: repo}
			got, ok := o.chartSpecForStep(step)
			require.True(t, ok)

			assert.Equal(t, want.Namespace, got.Namespace,
				"DB 가 비었으면 코드의 핀이 살아야 한다 — 비면 스택 네임스페이스로 설치된다")
		})
	}
}

// 마이그레이션을 전부 적용한 뒤의 DB 값이 코드의 핀과 같아야 한다.
//
// 000056 시드는 metrics-server 행의 namespace 를 NULL 로 두었다. 이미 적용된
// 마이그레이션은 고칠 수 없으니 뒤의 마이그레이션이 바로잡아야 한다 — 그것이
// 빠지면 기존 설치는 계속 스택 네임스페이스에 깔린다. 코드 폴백만으로는
// 운영자가 DB 를 읽을 때 어긋난 값을 본다.
func TestMigrationsEndWithSharedAddonNamespacePinned(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "..", "db", "migrations")
	files, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	require.NoError(t, err)
	require.NotEmpty(t, files)
	sort.Strings(files)

	for _, step := range []string{stepInstallingCertManager, "installing_metrics_server"} {
		t.Run(step, func(t *testing.T) {
			want, ok := defaultChartSpecForStep(step)
			require.True(t, ok)
			require.NotEmpty(t, want.Namespace, "이 테스트의 전제")

			effective, found := effectiveSeededNamespace(t, files, step)
			require.True(t, found, "어느 마이그레이션에서도 %s 의 namespace 를 정하지 않는다", step)

			assert.Equal(t, want.Namespace, effective,
				"마이그레이션을 다 적용한 뒤의 namespace 가 코드의 핀과 같아야 한다")
		})
	}
}

// effectiveSeededNamespace 는 마이그레이션을 파일 이름 순서로 훑어
// 그 단계에 마지막으로 지정된 namespace 를 돌려준다.
func effectiveSeededNamespace(t *testing.T, files []string, step string) (string, bool) {
	t.Helper()

	insertRow := regexp.MustCompile(`(?m)^\s*\('` + regexp.QuoteMeta(step) + `',\s*(?:NULL|'[^']*'),\s*'[^']*',\s*(?:NULL|'[^']*'),\s*(?:NULL|'[^']*'),\s*(NULL|'[^']*')`)
	update := regexp.MustCompile(`(?is)UPDATE\s+stack_helm_step_configs\s+SET\s+namespace\s*=\s*(NULL|'[^']*')[^;]*?step_name\s*=\s*'` + regexp.QuoteMeta(step) + `'`)

	value, found := "", false
	for _, f := range files {
		raw, err := os.ReadFile(f)
		require.NoError(t, err)
		sql := string(raw)

		if m := insertRow.FindStringSubmatch(sql); m != nil {
			value, found = unquoteSQL(m[1]), true
		}
		for _, m := range update.FindAllStringSubmatch(sql, -1) {
			value, found = unquoteSQL(m[1]), true
		}
	}
	return value, found
}

func unquoteSQL(v string) string {
	if v == "NULL" {
		return ""
	}
	return strings.Trim(v, "'")
}
