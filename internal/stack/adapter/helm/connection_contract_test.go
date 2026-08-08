package helm

import (
	"fmt"
	"testing"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// 설치 경로가 만드는 이름과 조회 경로(연결정보 안내)가 보는 이름이 같아야 한다.
//
// 두 값을 동시에 보는 테스트가 없어서 한쪽만 바뀌어도 컴파일과 기존 테스트가
// 모두 통과했고, 화면은 존재하지 않는 이름을 계속 안내했다. 이 파일이 그
// 커플링을 기계적으로 고정한다 — domain 상수를 바꾸면 설치 경로도 같이 바뀌어야
// 하고, 안 바꾸면 여기서 깨진다.

func TestChartReleaseNamesMatchDomainServiceNames(t *testing.T) {
	// Service 이름은 Helm 릴리스명에서 나온다. 연결정보의 endpoint 가
	// domain.PostgresServiceName / MinIOServiceName 을 쓰므로 릴리스명이
	// 달라지면 안내하는 주소가 곧바로 틀어진다.
	cases := []struct {
		step string
		want string
	}{
		{step: "installing_postgresql", want: domain.PostgresServiceName},
		{step: "installing_minio", want: domain.MinIOServiceName},
	}

	for _, tc := range cases {
		spec, ok := defaultChartSpecForStep(tc.step)
		if !ok {
			t.Fatalf("%s: chart spec not found", tc.step)
		}
		if spec.ReleaseName != tc.want {
			t.Errorf("%s: ReleaseName = %q, want %q (domain 상수와 어긋나면 연결정보가 잘못된 주소를 안내한다)",
				tc.step, spec.ReleaseName, tc.want)
		}
	}
}

func TestGitLabValuesMatchDomainPostgresConstants(t *testing.T) {
	values := DefaultValues("installing_gitlab")

	global, ok := values["global"].(map[string]any)
	if !ok {
		t.Fatal("global 블록이 없다")
	}
	psql, ok := global["psql"].(map[string]any)
	if !ok {
		t.Fatal("global.psql 블록이 없다")
	}

	wantHost := fmt.Sprintf("%s.%s.svc.cluster.local", domain.PostgresServiceName, defaultStackNamespace)
	if got := psql["host"]; got != wantHost {
		t.Errorf("psql.host = %v, want %v", got, wantHost)
	}
	if got := psql["port"]; got != domain.PostgresServicePort {
		t.Errorf("psql.port = %v, want %v", got, domain.PostgresServicePort)
	}
	if got := psql["database"]; got != domain.PostgresAppDatabase {
		t.Errorf("psql.database = %v, want %v", got, domain.PostgresAppDatabase)
	}
	if got := psql["username"]; got != domain.PostgresAppUser {
		t.Errorf("psql.username = %v, want %v", got, domain.PostgresAppUser)
	}

	password, ok := psql["password"].(map[string]any)
	if !ok {
		t.Fatal("global.psql.password 블록이 없다")
	}
	if got := password["secret"]; got != domain.ProvisionedPostgresSecret {
		t.Errorf("psql.password.secret = %v, want %v", got, domain.ProvisionedPostgresSecret)
	}
	if got := password["key"]; got != domain.PostgresPasswordKey {
		t.Errorf("psql.password.key = %v, want %v", got, domain.PostgresPasswordKey)
	}
}

func TestMinIOValuesMatchDomainSecret(t *testing.T) {
	values := DefaultValues("installing_minio")
	if got := values["existingSecret"]; got != domain.ProvisionedMinIOSecret {
		t.Errorf("existingSecret = %v, want %v", got, domain.ProvisionedMinIOSecret)
	}
}

func TestProvisionedMinIOSecretKeysMatchDomain(t *testing.T) {
	// 차트의 existingSecret 이 읽는 키와 연결정보가 안내하는 키가 같아야 한다.
	var entries []SecretEntry
	for _, managed := range managedSecrets("nullus") {
		if managed.TargetSecret == domain.ProvisionedMinIOSecret {
			entries = managed.Entries
			break
		}
	}
	if entries == nil {
		t.Fatalf("%s 를 만드는 ManagedSecret 이 없다", domain.ProvisionedMinIOSecret)
	}

	keys := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		keys[entry.TargetKey] = struct{}{}
	}
	for _, want := range []string{domain.MinIOUserKey, domain.MinIOPasswordKey} {
		if _, ok := keys[want]; !ok {
			t.Errorf("%s 에 %q 키가 없다 — 연결정보가 안내하는 키와 어긋난다",
				domain.ProvisionedMinIOSecret, want)
		}
	}
}

func TestGitLabRequiredBucketsContainArtifactsBucket(t *testing.T) {
	// 연결정보는 오브젝트 스토리지의 resource_name 으로 이 버킷을 안내한다.
	// 부트스트랩이 만들지 않는 버킷을 안내하면 그대로 실패한다.
	for _, bucket := range gitLabRequiredBuckets {
		if bucket == domain.GitLabArtifactsBucket {
			return
		}
	}
	t.Errorf("gitLabRequiredBuckets 에 %q 가 없다", domain.GitLabArtifactsBucket)
}
