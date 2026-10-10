package helm

// SonarQube 를 쓸 수 있는 상태로 만든다.
//
// 차트만으로는 두 가지가 빠진다.
//
//   - DB: 스택의 공유 PostgreSQL 은 GitLab 몫(gitlabhq_production)만 만든다. SonarQube 의
//     테이블(users·projects·issues)은 GitLab 과 이름이 겹쳐 같은 DB 를 쓸 수 없으므로,
//     설치 전에 전용 role 과 DB 를 만든다.
//   - 관리자 비밀번호: 차트의 setAdminPassword 훅은 curl 에 -f 가 없어 변경이 거부돼도
//     성공으로 끝난다(비밀번호 정책 위반 등). 그러면 관리자가 조용히 admin/admin 으로
//     남는다. 그래서 그 훅을 쓰지 않고 설치 뒤에 직접 바꾸고, 실패하면 단계를 멈춘다.
//
// 비밀번호는 매니페스트에 적지 않는다. 적으면 helm 히스토리와 이벤트에 평문으로 남는다.

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
	"github.com/cloud-nullus/draft/internal/stack/domain"
)

const (
	sonarqubeDatabaseJobName  = "nullus-sonarqube-database"
	sonarqubeProvisionJobName = "nullus-sonarqube-bootstrap"
	// sonarqubeAdminPasswordSuffix 는 생성기 값(영숫자)에 붙여 SonarQube 비밀번호 정책
	// (12자 이상 · 대문자 · 소문자 · 숫자 · 특수문자)을 늘 만족하게 한다.
	sonarqubeAdminPasswordSuffix = "-Sq9"
)

// sonarQubeJDBCURL 은 스택 네임스페이스의 공유 PostgreSQL 안 SonarQube 전용 DB 주소다.
func sonarQubeJDBCURL(namespace string) string {
	return fmt.Sprintf("jdbc:postgresql://%s.%s.svc.cluster.local:%d/%s",
		domain.PostgresServiceName, namespace, domain.PostgresServicePort, domain.SonarQubeDatabase)
}

// deriveSonarQubeAdminPassword 는 생성기 값에서 SonarQube 정책을 맞춘 관리자 비밀번호를 만든다.
func deriveSonarQubeAdminPassword(source string) (string, error) {
	source = strings.TrimSpace(source)
	if len(source) < 8 {
		return "", fmt.Errorf("sonarqube 관리자 비밀번호의 원본이 너무 짧습니다(%d자)", len(source))
	}
	return source + sonarqubeAdminPasswordSuffix, nil
}

// sonarQubeStackPostgresError 는 SonarQube 가 공유할 PostgreSQL 이 스택 안에 없으면 이유를 돌려준다.
//
// 외부 DB 를 고른 스택에는 nullus-postgresql 이 없다. 그대로 설치하면 JDBC 주소가 없는
// 서비스를 가리켜 파드가 기동하지 못하고, 원인은 SonarQube 로그 깊숙이에만 남는다.
func sonarQubeStackPostgresError(cfg domain.StackConfig) error {
	if cfg.Storage == nil || strings.TrimSpace(cfg.Storage.Database.Mode) == "create" {
		return nil
	}
	return fmt.Errorf("SonarQube 는 스택이 만든 PostgreSQL 안에 전용 DB 를 만들어 씁니다 — "+
		"DB 를 외부 연결(%s)로 고른 스택에는 설치할 수 없습니다", strings.TrimSpace(cfg.Storage.Database.Mode))
}

// sonarqubeDatabaseManifest 는 공유 PostgreSQL 에 SonarQube 전용 role 과 DB 를 만드는 Job 이다.
//
// 다시 돌려도 같은 결과다(재시도·재배포). role 은 없을 때만 만들고 비밀번호는 매번
// Secret 값으로 맞춘다 — 금고가 새로 초기화돼 값이 바뀌어도 둘이 갈라지지 않는다.
func sonarqubeDatabaseManifest(namespace, stackName string) string {
	ns := strings.TrimSpace(namespace)
	if ns == "" {
		ns = defaultStackNamespace
	}
	host := fmt.Sprintf("%s.%s.svc.cluster.local", domain.PostgresServiceName, ns)
	role, db := domain.SonarQubeDBUser, domain.SonarQubeDatabase

	// CREATE DATABASE 는 트랜잭션 안에서 돌 수 없다. \gexec 는 결과 행을 문장으로 하나씩
	// 실행하므로 "없을 때만" 조건을 걸면서도 트랜잭션 밖에서 돈다.
	script := strings.Join([]string{
		"set -e",
		fmt.Sprintf("until pg_isready -h %s -p %d -U postgres >/dev/null 2>&1; do", host, domain.PostgresServicePort),
		"  echo 'waiting for postgresql...'",
		"  sleep 3",
		"done",
		fmt.Sprintf(`psql -h %s -p %d -U postgres -d postgres -v ON_ERROR_STOP=1 -v pw="$SQ_PASSWORD" <<'SQL'`,
			host, domain.PostgresServicePort),
		fmt.Sprintf(`SELECT 'CREATE ROLE "%s" LOGIN' WHERE NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '%s')\gexec`, role, role),
		fmt.Sprintf(`ALTER ROLE "%s" WITH LOGIN PASSWORD :'pw';`, role),
		fmt.Sprintf(`SELECT 'CREATE DATABASE %s OWNER %s ENCODING ''UTF8'' TEMPLATE template0' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = '%s')\gexec`, db, role, db),
		"SQL",
		fmt.Sprintf("echo 'database %s ready'", db),
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
      - name: database
        image: %s
        env:
        - name: PGPASSWORD
          valueFrom:
            secretKeyRef:
              name: %s
              key: postgres-password
        - name: SQ_PASSWORD
          valueFrom:
            secretKeyRef:
              name: %s
              key: %s
        command: ["/bin/sh", "-c"]
        args:
          - |
%s
`, sonarqubeDatabaseJobName, ns, stackName, stackName, stackPostgresImageRef(),
		domain.ProvisionedPostgresSecret, domain.SonarQubeSecret, domain.SonarQubeDBPasswordKey,
		indentYAML(script, 12))
}

// ensureSonarQubeDatabase 는 설치 전에 전용 DB 를 만든다. 실패하면 설치를 멈춘다 —
// 넘어가면 SonarQube 가 DB 에 붙지 못해 재시작을 반복하고, 원인이 멀리 떨어진다.
func (o *Orchestrator) ensureSonarQubeDatabase(ctx context.Context, namespace string) error {
	stackName := strings.TrimSpace(namespace)
	if cfg := o.currentStackConfig(); cfg != nil {
		if err := sonarQubeStackPostgresError(*cfg); err != nil {
			return err
		}
		if domainName := strings.TrimSpace(cfg.AccessDomain); domainName != "" {
			stackName = domainName
		}
	}

	_, _ = o.runKubectl(ctx, "delete", "job", sonarqubeDatabaseJobName, "-n", namespace, "--ignore-not-found=true")
	if err := o.applyManifest(ctx, namespace, sonarqubeDatabaseManifest(namespace, stackName)); err != nil {
		return fmt.Errorf("sonarqube DB 준비 Job 생성 실패: %w", err)
	}
	if _, err := o.runKubectl(ctx, "wait", "-n", namespace,
		"--for=condition=complete", "--timeout=180s", "job/"+sonarqubeDatabaseJobName); err != nil {
		logs, _ := o.runKubectl(ctx, "logs", "-n", namespace, "job/"+sonarqubeDatabaseJobName, "--tail=20")
		return fmt.Errorf("공유 PostgreSQL 에 SonarQube DB 를 만들지 못했습니다: %w (%s)", err, strings.TrimSpace(string(logs)))
	}
	slog.Info("sonarqube database ready", "namespace", namespace, "database", domain.SonarQubeDatabase)
	_, _ = o.runKubectl(ctx, "delete", "job", sonarqubeDatabaseJobName, "-n", namespace, "--ignore-not-found=true")
	return nil
}

// sonarqubeProvisionManifest 는 기동을 기다린 뒤 관리자 비밀번호를 첫 값(admin)에서 바꾸는 Job 이다.
//
// 이미 바뀌었으면 건너뛴다 — 재시도·재배포에서 같은 Job 이 다시 돈다. 모든 호출에 -f 를
// 붙여 4xx 를 실패로 받는다.
func sonarqubeProvisionManifest(namespace, stackName string) string {
	ns := strings.TrimSpace(namespace)
	if ns == "" {
		ns = defaultStackNamespace
	}
	base := fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", domain.SonarQubeServiceName, ns, domain.SonarQubeServicePort)
	admin := domain.SonarQubeAdminUser

	script := strings.Join([]string{
		"set -eu",
		fmt.Sprintf("BASE=%s", base),
		"i=0",
		`until curl -fsS "$BASE/api/system/status" 2>/dev/null | grep -q '"status":"UP"'; do`,
		`  i=$((i+1))`,
		`  if [ "$i" -gt 120 ]; then echo 'sonarqube did not become UP'; exit 1; fi`,
		"  echo 'waiting for sonarqube...'",
		"  sleep 5",
		"done",
		fmt.Sprintf(`if curl -fsS -u "%s:$NEW_PASSWORD" "$BASE/api/authentication/validate" | grep -q '"valid":true'; then`, admin),
		"  echo 'admin password already set'",
		"  exit 0",
		"fi",
		fmt.Sprintf(`curl -fsS -u %s:%s -X POST "$BASE/api/users/change_password" \`, admin, admin),
		fmt.Sprintf(`  --data-urlencode "login=%s" --data-urlencode "previousPassword=%s" --data-urlencode "password=$NEW_PASSWORD"`, admin, admin),
		fmt.Sprintf(`curl -fsS -u "%s:$NEW_PASSWORD" "$BASE/api/authentication/validate" | grep -q '"valid":true'`, admin),
		"echo 'admin password set'",
	}, "\n")

	return fmt.Sprintf(`apiVersion: batch/v1
kind: Job
metadata:
  name: %s
  namespace: %s
  labels:
    nullus.io/stack-name: %s
spec:
  backoffLimit: 2
  ttlSecondsAfterFinished: 300
  template:
    metadata:
      labels:
        nullus.io/stack-name: %s
    spec:
      restartPolicy: Never
      containers:
      - name: bootstrap
        image: %s
        env:
        - name: NEW_PASSWORD
          valueFrom:
            secretKeyRef:
              name: %s
              key: %s
        command: ["/bin/sh", "-c"]
        args:
          - |
%s
`, sonarqubeProvisionJobName, ns, stackName, stackName, shareddomain.CurlImage,
		domain.SonarQubeSecret, domain.SonarQubeAdminPasswordKey, indentYAML(script, 12))
}

// ensureSonarQubeProvisioned 는 위 Job 을 돌린다. 첫 기동은 DB 스키마를 만드느라 몇 분
// 걸리므로 Job 안에서 최대 10분을 기다리고, 여기서는 그보다 길게 기다린다.
func (o *Orchestrator) ensureSonarQubeProvisioned(ctx context.Context, namespace string) error {
	stackName := strings.TrimSpace(namespace)
	if cfg := o.currentStackConfig(); cfg != nil && strings.TrimSpace(cfg.AccessDomain) != "" {
		stackName = strings.TrimSpace(cfg.AccessDomain)
	}

	_, _ = o.runKubectl(ctx, "delete", "job", sonarqubeProvisionJobName, "-n", namespace, "--ignore-not-found")
	if err := o.applyManifest(ctx, namespace, sonarqubeProvisionManifest(namespace, stackName)); err != nil {
		return fmt.Errorf("sonarqube 프로비저닝 Job 생성 실패: %w", err)
	}
	if _, err := o.runKubectl(ctx, "wait", "-n", namespace,
		"--for=condition=complete", "--timeout=900s", "job/"+sonarqubeProvisionJobName); err != nil {
		logs, _ := o.runKubectl(ctx, "logs", "-n", namespace, "job/"+sonarqubeProvisionJobName, "--tail=30")
		return fmt.Errorf("sonarqube 관리자 비밀번호를 바꾸지 못했습니다: %w (로그: %s)", err, strings.TrimSpace(string(logs)))
	}
	// CI 의 소스 정적 분석 단계가 쓸 토큰을 발급해 둔다(sonarqube-analysis-token.go).
	return o.ensureSonarQubeAnalysisToken(ctx, namespace, stackName)
}

// sonarqubeSharedServiceValues 는 네임스페이스와 접속 도메인에서 파생되는 값이다.
//
// JDBC 주소는 스택 네임스페이스의 공유 PostgreSQL 을 가리켜야 하고, 서버 주소는 화면
// 링크와 SAML 응답 주소(ACS)의 기준이 된다.
func (o *Orchestrator) sonarqubeSharedServiceValues() map[string]any {
	namespace := strings.TrimSpace(o.namespace)
	if namespace == "" {
		namespace = defaultStackNamespace
	}
	values := map[string]any{
		"jdbcOverwrite": map[string]any{"jdbcUrl": sonarQubeJDBCURL(namespace)},
	}
	if cfg := o.currentStackConfig(); cfg != nil && strings.TrimSpace(cfg.AccessDomain) != "" {
		values["sonarProperties"] = map[string]any{
			"sonar.core.serverBaseURL": fmt.Sprintf("%s://sonarqube.%s", o.toolURLScheme(), strings.TrimSpace(cfg.AccessDomain)),
		}
	}
	return values
}
