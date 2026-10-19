package helm

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
	"github.com/cloud-nullus/draft/internal/stack/domain"
)

func seaweedFSStackConfig() domain.StackConfig {
	cfg := domain.StackConfig{AccessDomain: "nullus.local"}
	cfg.Artifacts.SourceRepository = domain.ToolSelection{Name: "GitLab", Enabled: true}
	cfg.Artifacts.StorageBackend = domain.ToolSelection{Name: "SeaweedFS", Version: domain.SeaweedFSAppVersion, Enabled: true}
	return cfg
}

func TestDefaultChartSpecForStep_SeaweedFS(t *testing.T) {
	spec, ok := DefaultChartSpecForStep("installing_seaweedfs")

	require.True(t, ok, "installing_seaweedfs 에 차트 스펙이 없으면 스텝이 unknown step 으로 죽는다")
	assert.Equal(t, domain.SeaweedFSReleaseName, spec.ReleaseName)
	assert.Equal(t, "seaweedfs", spec.ChartName)
	assert.Equal(t, domain.SeaweedFSChartRepo, spec.RepoURL)
	assert.Equal(t, domain.SeaweedFSChartVersion, spec.Version)
}

// 오브젝트 스토리지 슬롯은 구현체를 하나만 세운다. 둘이 함께 서면 GitLab 은 한쪽만
// 보는데 다른 쪽이 자원과 PVC 를 먹는다.
func TestObjectStorageSteps_SelectExactlyOneBackend(t *testing.T) {
	tests := []struct {
		name          string
		backend       domain.ToolSelection
		wantMinIO     bool
		wantSeaweedFS bool
		wantConsumers bool
	}{
		{"SeaweedFS 를 고르면 SeaweedFS 만 선다", domain.ToolSelection{Name: "SeaweedFS", Enabled: true}, false, true, true},
		{"MinIO 를 고르면 MinIO 만 선다", domain.ToolSelection{Name: "MinIO", Enabled: true}, true, false, true},
		{"이름이 비면 기존처럼 MinIO 다", domain.ToolSelection{Enabled: true}, true, false, true},
		{"고르지 않으면 둘 다 서지 않는다", domain.ToolSelection{}, false, false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := seaweedFSStackConfig()
			cfg.Artifacts.StorageBackend = tc.backend
			o := NewOrchestrator(nil, []byte("not-a-kubeconfig"), "nullus")
			o.SetStackConfig(cfg)

			assert.Equal(t, tc.wantMinIO, o.IsStepEnabled("installing_minio"))
			assert.Equal(t, tc.wantSeaweedFS, o.IsStepEnabled("installing_seaweedfs"))
			// 뒤 단계는 어느 구현체가 섰는지 모른 채 S3 계약만 본다.
			assert.Equal(t, tc.wantConsumers, o.IsStepEnabled("installing_object_storage_secret"))
			// 버킷 단계는 kubeconfig 가 없으면 따로 꺼지므로 술어를 직접 본다.
			assert.Equal(t, tc.wantConsumers, o.stepConfigEnabled["installing_object_storage_buckets"](cfg))
		})
	}
}

// 설정을 모를 때 켜 두면 기본값(MinIO)과 함께 SeaweedFS 까지 선다.
func TestSeaweedFSStep_IsOptIn(t *testing.T) {
	assert.True(t, isOptInStep("installing_seaweedfs"))
	o := NewOrchestrator(nil, []byte("not-a-kubeconfig"), "nullus")
	assert.False(t, o.IsStepEnabled("installing_seaweedfs"))
}

func TestSeaweedFSStep_MapsToStorageSlotAndRelease(t *testing.T) {
	assert.Equal(t, domain.SlotStorageBackend, plannedSlotForStep["installing_seaweedfs"])
	assert.Equal(t, "installing_seaweedfs", releaseStepNames[domain.SeaweedFSReleaseName])

	o := NewOrchestrator(&mockInstaller{}, nil, "nullus")
	cfg := seaweedFSStackConfig()
	assert.Equal(t, "seaweedfs", o.resourceDefaultKeyForStep("installing_seaweedfs", &cfg))
}

// all-in-one 은 개별 컴포넌트를 끄지 않는다 — 넷을 켜 둔 채 두면 같은 역할의
// StatefulSet 이 따로 떠 자원을 두 배로 먹는다. S3 신원은 금고에서 렌더한 Secret 이다.
func TestSeaweedFSDefaultValues_AllInOneWithProvisionedIdentity(t *testing.T) {
	values := DefaultValues("installing_seaweedfs")
	require.NotNil(t, values)

	for _, component := range []string{"master", "volume", "filer", "s3"} {
		block, _ := values[component].(map[string]any)
		assert.Equalf(t, false, block["enabled"], "%s 가 켜져 있다", component)
	}
	allInOne, _ := values["allInOne"].(map[string]any)
	require.NotNil(t, allInOne)
	assert.Equal(t, true, allInOne["enabled"])

	s3, _ := allInOne["s3"].(map[string]any)
	assert.Equal(t, true, s3["enabled"])
	assert.Equal(t, true, s3["enableAuth"], "인증을 끄면 누구나 버킷을 읽고 쓴다")
	assert.Equal(t, domain.ProvisionedSeaweedFSSecret, s3["existingConfigSecret"])

	data, _ := allInOne["data"].(map[string]any)
	assert.Equal(t, "persistentVolumeClaim", data["type"], "emptyDir 이면 재시작에 오브젝트가 사라진다")

	global, _ := values["global"].(map[string]any)
	seaweed, _ := global["seaweedfs"].(map[string]any)
	assert.Equal(t, false, seaweed["createClusterRole"],
		"ClusterRole 이름이 릴리스명으로 고정이라 켜 두면 두 번째 스택이 소유권 충돌로 막힌다")

	image, _ := values["image"].(map[string]any)
	assert.Equal(t, shareddomain.SeaweedFSImage, image["repository"].(string)+":"+image["tag"].(string))
	assert.True(t, strings.HasSuffix(shareddomain.SeaweedFSImage, ":"+domain.SeaweedFSAppVersion),
		"이미지 태그가 차트 appVersion 과 갈라졌다")
}

// Storage 탭에서 고른 크기가 PVC 에 실려야 한다. 설치 후에 늘리기 어려운 값이다.
func TestSeaweedFSValues_UseSelectedObjectStorageSize(t *testing.T) {
	cfg := seaweedFSStackConfig()
	cfg.Storage = &domain.StorageConfig{ObjectStorage: domain.StorageTarget{Mode: "create", Size: 100}}
	o := NewOrchestrator(nil, []byte("not-a-kubeconfig"), "nullus")
	o.SetStackConfig(cfg)

	spec, ok := DefaultChartSpecForStep("installing_seaweedfs")
	require.True(t, ok)
	size, found := lookupValue(o.valuesForStep("installing_seaweedfs", spec), []string{"allInOne", "data", "size"})
	require.True(t, found)
	assert.Equal(t, "100Gi", size)
}

// SeaweedFS 를 고르면 비밀값 평면이 SeaweedFS 신원 Secret 을 만들고, GitLab 의 연결
// Secret 은 SeaweedFS 엔드포인트와 그 자격을 가리킨다. MinIO Secret 은 만들지 않는다.
func TestManagedSecretsFor_SeaweedFS(t *testing.T) {
	items := managedSecretsFor("team-a", domain.SeaweedFSObjectStorage)
	byName := map[string]ManagedSecret{}
	for _, item := range items {
		byName[item.TargetSecret] = item
	}

	assert.NotContains(t, byName, domain.ProvisionedMinIOSecret, "고르지 않은 구현체의 Secret 은 만들지 않는다")

	identity, ok := byName[domain.ProvisionedSeaweedFSSecret]
	require.True(t, ok, "SeaweedFS 신원 Secret 이 없으면 게이트웨이가 FailedMount 로 서지 못한다")
	assert.True(t, identity.KeepEntries, "원본 키를 남기지 않으면 버킷 Job 이 자격을 읽지 못한다")
	paths := map[string]string{}
	for _, e := range identity.Entries {
		paths[e.TargetKey] = e.PathSuffix
	}
	assert.Equal(t, "artifacts/seaweedfs/access-key", paths[domain.SeaweedFSAccessKeyKey])
	assert.Equal(t, "artifacts/seaweedfs/secret-key", paths[domain.SeaweedFSSecretKeyKey])

	// ESO 가 채울 자리({{ .키 }})를 가짜 값으로 바꿔 JSON 으로 읽혀야 한다.
	rendered := strings.NewReplacer(
		"{{ .accessKey }}", "ak", "{{ .secretKey }}", "sk",
	).Replace(identity.TemplateData[domain.SeaweedFSS3ConfigKey])
	var config struct {
		Identities []struct {
			Name        string `json:"name"`
			Credentials []struct {
				AccessKey string `json:"accessKey"`
				SecretKey string `json:"secretKey"`
			} `json:"credentials"`
			Actions []string `json:"actions"`
		} `json:"identities"`
	}
	require.NoError(t, json.Unmarshal([]byte(rendered), &config), rendered)
	require.Len(t, config.Identities, 1)
	assert.Equal(t, "ak", config.Identities[0].Credentials[0].AccessKey)
	assert.Equal(t, "sk", config.Identities[0].Credentials[0].SecretKey)
	assert.Contains(t, config.Identities[0].Actions, "Admin")

	endpoint := domain.SeaweedFSObjectStorage.Endpoint("team-a")
	for _, consumer := range []string{ProvisionedObjectStorageSecret, ProvisionedRegistryStorageSecret} {
		item, ok := byName[consumer]
		require.Truef(t, ok, "%s 가 없다", consumer)
		for _, tpl := range item.TemplateData {
			assert.Containsf(t, tpl, endpoint, "%s 가 SeaweedFS 를 가리키지 않는다", consumer)
		}
		for _, e := range item.Entries {
			assert.Truef(t, strings.HasPrefix(e.PathSuffix, "artifacts/seaweedfs/"),
				"%s 가 다른 구현체의 자격(%s)을 읽는다", consumer, e.PathSuffix)
		}
	}
}

// 기존(MinIO) 스택이 보던 Secret 은 그대로다 — 이름이나 경로가 바뀌면 재설치가 새
// 비밀번호를 만들어 이미 쌓인 오브젝트에 못 붙는다.
func TestManagedSecrets_DefaultStaysMinIO(t *testing.T) {
	items := managedSecrets("nullus")
	byName := map[string]ManagedSecret{}
	for _, item := range items {
		byName[item.TargetSecret] = item
	}
	minio, ok := byName[domain.ProvisionedMinIOSecret]
	require.True(t, ok)
	assert.False(t, minio.KeepEntries)
	assert.Empty(t, minio.TemplateData, "MinIO Secret 모양이 바뀌면 이미 선 스택의 Secret 이 다시 써진다")
	assert.NotContains(t, byName, domain.ProvisionedSeaweedFSSecret)
}

func TestObjectStorageBackend_FollowsStackConfig(t *testing.T) {
	o := NewOrchestrator(nil, []byte("not-a-kubeconfig"), "nullus")
	o.SetStackConfig(seaweedFSStackConfig())
	assert.Equal(t, domain.ObjectStorageToolSeaweedFS, o.objectStorageBackend().Tool)

	o.SetStackConfig(domain.StackConfig{})
	assert.Equal(t, domain.ObjectStorageToolMinIO, o.objectStorageBackend().Tool,
		"설정을 모르면 기존 기본값인 MinIO 다")
}

// SeaweedFS 는 화면(콘솔)이 없다 — 게이트웨이에 minio 라우트를 만들 이유가 없다.
func TestDefaultGatewayBundleManifest_NoConsoleRouteForSeaweedFS(t *testing.T) {
	o := NewOrchestrator(&mockInstaller{}, []byte("kubeconfig"), "nullus-demo")
	o.SetStackConfig(seaweedFSStackConfig())

	manifest := o.defaultGatewayBundleManifest("nullus-demo")
	assert.NotContains(t, manifest, "minio.nullus.local")
	assert.NotContains(t, manifest, domain.SeaweedFSServiceName)
}

const liveLikeKubeconfig = "apiVersion: v1\nkind: Config\nclusters: []\n"

// 이미 다른 구현체로 선 스택에서 구현체를 바꾸면 GitLab 의 연결 Secret 이 빈 새 스토리지를
// 가리키게 되고 기존 아티팩트·업로드·이미지가 보이지 않는다(옛 릴리스는 고아로 남는다).
// 데이터를 옮기는 기능이 없으므로 설치 단계가 이유를 남기고 멈춘다.
func TestObjectStorageStep_RefusesToSwitchBackendOnInstalledStack(t *testing.T) {
	installer := &mockInstaller{strictStatus: true, installed: []string{domain.MinIOServiceName}}
	o := NewOrchestrator(installer, []byte(liveLikeKubeconfig), "nullus")
	o.SetStackConfig(seaweedFSStackConfig())

	err := o.ensureObjectStorageBackendNotSwitched(context.Background(), domain.SeaweedFSObjectStorage, "nullus")
	require.Error(t, err)
	assert.Contains(t, err.Error(), domain.MinIOServiceName)

	err = o.ExecuteStep(context.Background(), "", "installing_seaweedfs", "A")
	require.Error(t, err, "가드를 지나 SeaweedFS 를 설치했다")
	assert.NotContains(t, installer.installed, domain.SeaweedFSReleaseName)
}

func TestObjectStorageStep_AllowsSameBackendAndFreshStack(t *testing.T) {
	// 같은 구현체의 재설치·재시도는 막지 않는다.
	same := &mockInstaller{strictStatus: true, installed: []string{domain.SeaweedFSReleaseName}}
	o := NewOrchestrator(same, []byte(liveLikeKubeconfig), "nullus")
	require.NoError(t, o.ensureObjectStorageBackendNotSwitched(context.Background(), domain.SeaweedFSObjectStorage, "nullus"))

	// 아무것도 없는 새 스택은 그대로 선다.
	fresh := &mockInstaller{strictStatus: true}
	o = NewOrchestrator(fresh, []byte(liveLikeKubeconfig), "nullus")
	require.NoError(t, o.ensureObjectStorageBackendNotSwitched(context.Background(), domain.MinIOObjectStorage, "nullus"))

	// 확인 자체가 실패하면(클러스터 오류) 막지 않는다 — 설치가 그 자리에서 진짜 이유로 실패한다.
	broken := &mockInstaller{failStatus: errors.New("connection refused")}
	o = NewOrchestrator(broken, []byte(liveLikeKubeconfig), "nullus")
	require.NoError(t, o.ensureObjectStorageBackendNotSwitched(context.Background(), domain.MinIOObjectStorage, "nullus"))
}

// 가드는 연결 배선을 처음 건드리는 단계 앞에서 서야 한다. 구현체 단계에서만 막으면 그 앞의
// provisioning_secrets 가 이미 GitLab 연결 Secret 을 없는 구현체로 다시 렌더한 뒤다 — kind
// 실측에서 그렇게 nullus-object-storage 가 nullus-minio 를 가리킨 채로 남았다.
func TestProvisioningSecrets_RefusesBackendSwitchBeforeRewiring(t *testing.T) {
	installer := &mockInstaller{strictStatus: true, installed: []string{domain.SeaweedFSReleaseName}}
	o := NewOrchestrator(installer, []byte(liveLikeKubeconfig), "nullus")
	cfg := seaweedFSStackConfig()
	cfg.Artifacts.StorageBackend = domain.ToolSelection{Name: "MinIO", Enabled: true}
	o.SetStackConfig(cfg)

	err := o.ExecuteStep(context.Background(), "", "provisioning_secrets", "A")
	require.Error(t, err)
	assert.Contains(t, err.Error(), domain.SeaweedFSReleaseName)
}
