package helm

import (
	"strings"
	"testing"
	"unicode"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

func sastStackConfig() domain.StackConfig {
	cfg := domain.StackConfig{AccessDomain: "nullus.local"}
	cfg.Security.SAST = domain.ToolSelection{Name: "SonarQube", Version: domain.SonarQubeAppVersion, Enabled: true}
	return cfg
}

func TestDefaultChartSpecForStep_SonarQube(t *testing.T) {
	spec, ok := DefaultChartSpecForStep("installing_sonarqube")

	require.True(t, ok, "installing_sonarqube 에 차트 스펙이 없으면 스텝이 unknown step 으로 죽는다")
	assert.Equal(t, domain.SonarQubeReleaseName, spec.ReleaseName)
	assert.Equal(t, "sonarqube", spec.ChartName)
	assert.Equal(t, "https://SonarSource.github.io/helm-chart-sonarqube", spec.RepoURL)
	assert.Equal(t, domain.SonarQubeChartVersion, spec.Version)
}

// SonarQube 는 선택이다. 고르지 않은 스택에 서면 아무도 고르지 않은 무거운 워크로드가 뜬다.
func TestSonarQubeSteps_OnlyWhenSASTSelected(t *testing.T) {
	tests := []struct {
		name string
		sel  domain.ToolSelection
		want bool
	}{
		{"고르면 선다", domain.ToolSelection{Enabled: true, Name: "SonarQube"}, true},
		{"고르지 않으면 서지 않는다", domain.ToolSelection{}, false},
		{"외부 SonarQube 는 설치하지 않는다", domain.ToolSelection{Enabled: true, Name: "SonarQube", Version: "external"}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := domain.StackConfig{}
			cfg.Security.SAST = tc.sel
			o := NewOrchestrator(nil, []byte("not-a-kubeconfig"), "nullus")
			o.SetStackConfig(cfg)

			assert.Equal(t, tc.want, o.IsStepEnabled("installing_sonarqube"))
			assert.Equal(t, tc.want, o.IsStepEnabled("provisioning_sonarqube"))
		})
	}
}

// 설정을 모를 때 켜 두면 아무도 고르지 않은 SonarQube 가 선다.
func TestSonarQubeSteps_AreOptIn(t *testing.T) {
	assert.True(t, isOptInStep("installing_sonarqube"))
	assert.True(t, isOptInStep("provisioning_sonarqube"))
}

func TestSonarQubeStep_MapsToSASTSlot(t *testing.T) {
	assert.Equal(t, domain.SlotSAST, plannedSlotForStep["installing_sonarqube"])
}

func TestSonarQubeRelease_MapsToInstallStep(t *testing.T) {
	assert.Equal(t, "installing_sonarqube", releaseStepNames[domain.SonarQubeReleaseName])
}

// 차트는 에디션과 모니터링 패스코드가 없으면 렌더부터 실패한다. 비밀값은 values 에
// 평문으로 두지 않고 ESO 가 만든 Secret 을 가리킨다 — values 는 helm 히스토리에 남는다.
func TestSonarQubeDefaultValues(t *testing.T) {
	values := DefaultValues("installing_sonarqube")
	require.NotNil(t, values)

	community, _ := values["community"].(map[string]any)
	assert.Equal(t, true, community["enabled"], "에디션을 고르지 않으면 차트 렌더가 실패한다")
	assert.Equal(t, domain.SonarQubeServiceName, values["fullnameOverride"],
		"이름을 고정하지 않으면 Service 가 sonarqube-sonarqube 가 되어 게이트웨이가 찾지 못한다")
	assert.Equal(t, domain.SonarQubeSecret, values["monitoringPasscodeSecretName"])
	assert.Equal(t, domain.SonarQubeMonitoringPasscodeKey, values["monitoringPasscodeSecretKey"])

	jdbc, _ := values["jdbcOverwrite"].(map[string]any)
	assert.Equal(t, true, jdbc["enabled"], "끄면 내장 H2 로 떠서 데이터가 공유 DB·백업 밖에 놓인다")
	assert.Equal(t, domain.SonarQubeDBUser, jdbc["jdbcUsername"])
	assert.Equal(t, domain.SonarQubeSecret, jdbc["jdbcSecretName"])
	assert.Equal(t, domain.SonarQubeDBPasswordKey, jdbc["jdbcSecretPasswordKey"])

	persistence, _ := values["persistence"].(map[string]any)
	assert.Equal(t, true, persistence["enabled"], "검색 색인을 휘발 볼륨에 두면 재시작마다 다시 만든다")

	// kind 실측: 분석 없이 떠 있기만 해도 2.69Gi 를 쓴다. 요청이 그보다 작으면 노드 메모리
	// 압박 때 먼저 축출된다.
	requests, _ := lookupValue(values, []string{"resources", "requests", "memory"})
	limits, _ := lookupValue(values, []string{"resources", "limits", "memory"})
	assert.Equal(t, "3Gi", requests, "요청은 유휴 사용량(2.69Gi) 위여야 한다")
	assert.Equal(t, "4Gi", limits)

	for _, path := range [][]string{{"ingress", "enabled"}, {"httproute", "enabled"}, {"tests", "enabled"}, {"prometheusExporter", "enabled"}} {
		got, found := lookupValue(values, path)
		assert.True(t, found, strings.Join(path, "."))
		assert.Equal(t, false, got, strings.Join(path, "."))
	}

	for _, key := range []string{"jdbcPassword", "monitoringPasscode", "account", "setAdminPassword"} {
		_, top := values[key]
		_, nested := jdbc[key]
		assert.False(t, top || nested, "%s 는 values 에 두지 않는다(평문이 helm 히스토리에 남거나, 실패해도 성공으로 끝나는 훅이 돈다)", key)
	}
}

// JDBC 주소는 스택 네임스페이스에서 파생된다. 서버 접속 주소는 SAML·분석 링크가 쓴다.
func TestSonarQubeValues_PointAtStackPostgresAndAccessDomain(t *testing.T) {
	o := NewOrchestrator(&mockInstaller{}, []byte("kubeconfig"), "nullus-demo")
	o.SetStackConfig(sastStackConfig())
	spec, _ := DefaultChartSpecForStep("installing_sonarqube")

	values := o.valuesForStep("installing_sonarqube", spec)

	url, _ := lookupValue(values, []string{"jdbcOverwrite", "jdbcUrl"})
	assert.Equal(t, "jdbc:postgresql://nullus-postgresql.nullus-demo.svc.cluster.local:5432/sonarqube", url)
	base, _ := lookupValue(values, []string{"sonarProperties", "sonar.core.serverBaseURL"})
	assert.Equal(t, o.toolURLScheme()+"://sonarqube.nullus.local", base)
}

// 라이브 values 편집이 옛 네임스페이스의 JDBC 주소를 얼려 두면 다른 스택의 DB 를 가리킨다.
func TestSonarQubeJDBCURL_IsPlatformOwned(t *testing.T) {
	o := NewOrchestrator(&mockInstaller{}, []byte("kubeconfig"), "nullus-demo")
	o.SetStackConfig(sastStackConfig())

	values := o.enforcePlatformOwnedValues("installing_sonarqube", map[string]any{
		"jdbcOverwrite": map[string]any{"jdbcUrl": "jdbc:postgresql://nullus-postgresql.old-stack.svc.cluster.local:5432/sonarqube"},
	})

	url, _ := lookupValue(values, []string{"jdbcOverwrite", "jdbcUrl"})
	assert.Equal(t, "jdbc:postgresql://nullus-postgresql.nullus-demo.svc.cluster.local:5432/sonarqube", url)
}

// GitLab DB 와 테이블 이름이 겹쳐 같은 DB 를 쓸 수 없다. 공유 PostgreSQL 안에 전용
// role 과 DB 를 만든다. 다시 돌려도 같은 결과여야 하고(재시도·재배포), 비밀번호는
// 매니페스트에 적지 않는다.
func TestSonarQubeDatabaseManifest(t *testing.T) {
	manifest := sonarqubeDatabaseManifest("nullus-demo", "demo")

	assert.Contains(t, manifest, "kind: Job")
	assert.Contains(t, manifest, "nullus-postgresql.nullus-demo.svc.cluster.local")
	assert.Contains(t, manifest, `WHERE NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'sonarqube')\gexec`)
	assert.Contains(t, manifest, `ALTER ROLE "sonarqube" WITH LOGIN PASSWORD :'pw';`)
	assert.Contains(t, manifest, `WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'sonarqube')\gexec`)
	assert.Contains(t, manifest, "OWNER sonarqube")
	assert.Contains(t, manifest, "<<'SQL'", "셸 확장이 없어야 psql 이 :'pw' 를 변수로 읽는다")
	assert.Contains(t, manifest, "name: "+domain.SonarQubeSecret)
	assert.Contains(t, manifest, "key: "+domain.SonarQubeDBPasswordKey)
	assert.Contains(t, manifest, "key: postgres-password")
}

// 외부 DB 를 고른 스택에는 공유할 PostgreSQL 이 없다. 조용히 내장 DB 로 뜨거나 엉뚱한
// 곳에 붙지 않게 설치 단계에서 이유와 함께 멈춘다.
func TestSonarQubeNeedsStackPostgres(t *testing.T) {
	cfg := sastStackConfig()
	assert.NoError(t, sonarQubeStackPostgresError(cfg))

	cfg.Storage = &domain.StorageConfig{}
	cfg.Storage.Database.Mode = "existing-connect"
	err := sonarQubeStackPostgresError(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "PostgreSQL")
}

// 차트의 관리자 비밀번호 훅은 curl 이 4xx 를 받아도 성공으로 끝난다. 프로비저닝 Job 이
// 직접 바꾸고, 실패하면 단계가 실패해야 한다. 이미 바뀐 비밀번호면 다시 바꾸지 않는다.
func TestSonarQubeProvisionManifest(t *testing.T) {
	manifest := sonarqubeProvisionManifest("nullus-demo", "demo")

	assert.Contains(t, manifest, "kind: Job")
	assert.Contains(t, manifest, "api/system/status", "기동이 끝나기 전에 부르면 503 으로 실패한다")
	assert.Contains(t, manifest, "api/authentication/validate", "이미 바뀌었으면 건너뛰어야 재시도가 성공한다")
	assert.Contains(t, manifest, "api/users/change_password")
	assert.Contains(t, manifest, "curl -fsS", "-f 가 없으면 4xx 도 성공으로 끝난다")
	assert.Contains(t, manifest, "http://sonarqube.nullus-demo.svc.cluster.local:9000")
	assert.Contains(t, manifest, "name: "+domain.SonarQubeSecret)
	assert.Contains(t, manifest, "key: "+domain.SonarQubeAdminPasswordKey)
}

// SonarQube 는 12자 이상에 대문자·소문자·숫자·특수문자를 요구한다. 영숫자만 만드는
// 생성기 값을 그대로 쓰면 비밀번호 변경이 거부된다.
func TestSonarQubeAdminPassword_MeetsPasswordPolicy(t *testing.T) {
	got, err := deriveSonarQubeAdminPassword("abcdefghijklmnopqrstuvwx")
	require.NoError(t, err)

	var upper, lower, digit, special bool
	for _, r := range got {
		switch {
		case unicode.IsUpper(r):
			upper = true
		case unicode.IsLower(r):
			lower = true
		case unicode.IsDigit(r):
			digit = true
		default:
			special = true
		}
	}
	assert.GreaterOrEqual(t, len(got), 12)
	assert.True(t, upper && lower && digit && special, got)
}

// 비밀값 평면이 SonarQube 의 Secret 을 만든다. 관리자 비밀번호는 정책을 맞춘 값이다.
func TestManagedSecrets_IncludeSonarQube(t *testing.T) {
	var found *ManagedSecret
	for _, ms := range managedSecrets("nullus-demo") {
		if ms.TargetSecret == domain.SonarQubeSecret {
			ms := ms
			found = &ms
		}
	}
	require.NotNil(t, found, "SonarQube Secret 이 비밀값 평면에 없다")

	keys := map[string]SecretEntry{}
	for _, e := range found.Entries {
		keys[e.TargetKey] = e
	}
	assert.Contains(t, keys, domain.SonarQubeDBPasswordKey)
	assert.Contains(t, keys, domain.SonarQubeMonitoringPasscodeKey)
	require.Contains(t, keys, domain.SonarQubeAdminPasswordKey)
	assert.NotNil(t, keys[domain.SonarQubeAdminPasswordKey].Derive, "관리자 비밀번호는 정책을 맞춰 계산해야 한다")
}

// 화면이 있는 도구라 게이트웨이 라우트가 있어야 주소가 열린다.
func TestDefaultGatewayBundleManifest_RoutesSonarQube(t *testing.T) {
	o := NewOrchestrator(&mockInstaller{}, []byte("kubeconfig"), "nullus-demo")
	o.SetStackConfig(sastStackConfig())

	manifest := o.defaultGatewayBundleManifest("nullus-demo")

	assert.Contains(t, manifest, "sonarqube.nullus.local")
	assert.Contains(t, manifest, "name: "+domain.SonarQubeServiceName)
	assert.Contains(t, manifest, "port: 9000")
}

func TestSonarQubeResourceDefaults(t *testing.T) {
	o := NewOrchestrator(&mockInstaller{}, nil, "nullus-demo")
	cfg := sastStackConfig()
	assert.Equal(t, "sonarqube", o.resourceDefaultKeyForStep("installing_sonarqube", &cfg))
}
