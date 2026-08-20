package helm

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// 스택 PostgreSQL 이미지. Job 과 차트 values 가 같은 값을 봐야 한다 —
// 갈라지면 에어갭 번들에 없는 이미지를 끌어오게 된다.
const (
	stackPostgresImageRegistry   = "docker.io"
	stackPostgresImageRepository = "bitnamilegacy/postgresql"
	stackPostgresImageTag        = "17.6.0-debian-12-r4"
)

const postgresRoleSyncJobName = "nullus-postgresql-role-sync"

func stackPostgresImageRef() string {
	return fmt.Sprintf("%s/%s:%s", stackPostgresImageRegistry, stackPostgresImageRepository, stackPostgresImageTag)
}

// postgresRoleSyncManifest 는 앱 사용자의 비밀번호를 Secret 값에 맞추는 Job 이다.
//
// 비밀번호의 출처와 그것이 구워지는 곳의 수명이 다르기 때문에 필요하다.
// 출처는 OpenBao(→ ExternalSecrets → Secret)이고, PostgreSQL 은 데이터
// 디렉터리가 비어 있을 때 딱 한 번 그 값으로 사용자를 만든다. 그래서
//
//   - 볼륨이 남아 있는 채로 다시 설치하거나
//   - 금고가 새로 초기화되어 값이 새로 생성되면
//
// 둘이 조용히 갈라진다. 설치는 멈추지 않고 여섯 단계쯤 더 간 뒤, Gitea 가
// "password authentication failed for user %q (28P01)" 로 기동하지 못하는
// 모습으로 드러난다 — 원인에서 가장 먼 자리다(2026-08-20 운영에서 실측).
//
// 그래서 PostgreSQL 을 세운 직후에 맞춘다. 비밀번호는 매니페스트에 적지 않는다.
// 적으면 helm 히스토리와 이벤트에 평문으로 남는다.
func postgresRoleSyncManifest(namespace, stackName string) string {
	ns := strings.TrimSpace(namespace)
	if ns == "" {
		ns = defaultStackNamespace
	}
	host := fmt.Sprintf("%s.%s.svc.cluster.local", domain.PostgresServiceName, ns)

	// psql 의 변수 인용(:'pw')을 쓴다. SQL 문자열에 값을 이어 붙이면 따옴표가 든
	// 비밀번호에서 구문이 깨지고, -c 인자로 넘기면 ps 에도 보인다.
	// SQL 은 따옴표 없는 heredoc 이 아니라 <<'SQL' 로 넘긴다. 셸 확장이 없어야
	// psql 이 :'pw' 를 자기 변수 인용으로 읽는다.
	script := strings.Join([]string{
		"set -e",
		fmt.Sprintf("until pg_isready -h %s -p %d -U postgres >/dev/null 2>&1; do", host, domain.PostgresServicePort),
		"  echo 'waiting for postgresql...'",
		"  sleep 3",
		"done",
		fmt.Sprintf(`psql -h %s -p %d -U postgres -d postgres -v ON_ERROR_STOP=1 -v pw="$APP_PASSWORD" <<'SQL'`,
			host, domain.PostgresServicePort),
		fmt.Sprintf(`ALTER ROLE "%s" WITH PASSWORD :'pw';`, domain.PostgresAppUser),
		"SQL",
		fmt.Sprintf("echo 'role %s password synchronised'", domain.PostgresAppUser),
	}, "\n")

	return fmt.Sprintf(`apiVersion: batch/v1
kind: Job
metadata:
  name: %s
  namespace: %s
  labels:
    nullus.io/stack-name: %s
spec:
  backoffLimit: 3
  ttlSecondsAfterFinished: 300
  template:
    metadata:
      labels:
        nullus.io/stack-name: %s
    spec:
      restartPolicy: Never
      containers:
      - name: role-sync
        image: %s
        env:
        - name: PGPASSWORD
          valueFrom:
            secretKeyRef:
              name: %s
              key: postgres-password
        - name: APP_PASSWORD
          valueFrom:
            secretKeyRef:
              name: %s
              key: %s
        command: ["/bin/sh", "-c"]
        args:
          - |
%s
`, postgresRoleSyncJobName, ns, stackName, stackName, stackPostgresImageRef(),
		domain.ProvisionedPostgresSecret, domain.ProvisionedPostgresSecret, domain.PostgresPasswordKey,
		indentYAML(script, 12))
}

// ensureProvisionedPostgresRolePassword 는 위 Job 을 돌려 비밀번호를 맞춘다.
//
// 실패하면 설치를 멈춘다. 여기서 넘어가면 다음 도구가 대신 실패하는데, 그 자리에서
// 보이는 것은 "DB 인증 실패" 뿐이라 원인을 찾는 데 훨씬 오래 걸린다.
func (o *Orchestrator) ensureProvisionedPostgresRolePassword(ctx context.Context, namespace string) error {
	stackName := ""
	if cfg := o.currentStackConfig(); cfg != nil {
		stackName = strings.TrimSpace(cfg.AccessDomain)
	}
	if stackName == "" {
		stackName = strings.TrimSpace(namespace)
	}

	_, _ = o.runKubectl(ctx, "delete", "job", postgresRoleSyncJobName, "-n", namespace, "--ignore-not-found=true")
	if err := o.applyManifest(ctx, namespace, postgresRoleSyncManifest(namespace, stackName)); err != nil {
		return fmt.Errorf("apply postgres role sync job: %w", err)
	}

	if _, err := o.runKubectl(ctx, "wait", "-n", namespace,
		"--for=condition=complete", "--timeout=180s", "job/"+postgresRoleSyncJobName); err != nil {
		logs, _ := o.runKubectl(ctx, "logs", "-n", namespace, "job/"+postgresRoleSyncJobName, "--tail=20")
		return fmt.Errorf(
			"PostgreSQL 앱 사용자(%s)의 비밀번호를 Secret 값과 맞추지 못했습니다: %w (%s)",
			domain.PostgresAppUser, err, strings.TrimSpace(string(logs)))
	}

	slog.Info("postgres role password synchronised with provisioned secret",
		"namespace", namespace, "role", domain.PostgresAppUser)
	_, _ = o.runKubectl(ctx, "delete", "job", postgresRoleSyncJobName, "-n", namespace, "--ignore-not-found=true")
	return nil
}
