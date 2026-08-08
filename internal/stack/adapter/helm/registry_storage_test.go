package helm

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

// GitLab Container Registry 는 기본값이 filesystem(/tmp/registry) 이라
// 파드 재시작 시 이미지가 사라지고, replica 가 2개면 push/pull 이 서로 다른
// 파드에 걸려 비결정적으로 실패한다. MinIO(S3) 백엔드로 고정한다.

func TestGitLabRequiredBuckets_IncludesRegistryBucket(t *testing.T) {
	assert.Contains(t, gitLabRequiredBuckets, RegistryStorageBucket,
		"registry 전용 버킷이 없으면 S3 백엔드로 전환할 수 없다")
}

func TestRegistryStorageConfigTemplate_RendersDockerDistributionS3(t *testing.T) {
	tpl := registryStorageConfigTemplate("http://nullus-minio.devsecops.svc.cluster.local:9000")

	// Docker distribution 의 storage 블록 형식이어야 한다.
	// (GitLab appConfig 의 object_store 형식과 키 이름이 다르다)
	assert.Contains(t, tpl, "s3:")
	assert.Contains(t, tpl, "bucket: "+RegistryStorageBucket)
	assert.Contains(t, tpl, "accesskey: {{ .accessKey }}")
	assert.Contains(t, tpl, "secretkey: {{ .secretKey }}")
	assert.Contains(t, tpl, "regionendpoint: http://nullus-minio.devsecops.svc.cluster.local:9000")
	assert.Contains(t, tpl, "region: us-east-1")
	assert.Contains(t, tpl, "v4auth: true")
	assert.Contains(t, tpl, "pathstyle: true")

	// 리다이렉트가 켜져 있으면 레지스트리가 클라이언트를 MinIO 의 클러스터 내부
	// 주소로 보내는데, kubelet 은 클러스터 DNS 를 쓰지 않아 해석하지 못한다.
	// 실제로 이미지 pull 이 i/o timeout 으로 실패했다.
	assert.Contains(t, tpl, "redirect:")
	assert.Contains(t, tpl, "disable: true")
}

func TestManagedSecrets_IncludesRegistryStorageSecret(t *testing.T) {
	items := managedSecrets("devsecops")

	var found *ManagedSecret
	for i := range items {
		if items[i].TargetSecret == ProvisionedRegistryStorageSecret {
			found = &items[i]
			break
		}
	}
	require.NotNil(t, found, "registry storage 시크릿이 프로비저닝 목록에 없다")

	assert.Contains(t, found.TemplateData, RegistryStorageSecretKey)
	assert.Contains(t, found.TemplateData[RegistryStorageSecretKey], "bucket: "+RegistryStorageBucket)

	// MinIO 루트 자격을 그대로 재사용한다 — 별도 자격을 만들면 버킷 정책까지
	// 관리해야 하므로 오브젝트 스토리지 시크릿과 같은 경로를 본다.
	keys := map[string]bool{}
	for _, e := range found.Entries {
		keys[e.TargetKey] = true
	}
	assert.True(t, keys["accessKey"], "accessKey 엔트리가 필요하다")
	assert.True(t, keys["secretKey"], "secretKey 엔트리가 필요하다")

	// 회전 후 registry 파드가 설정을 다시 읽어야 한다.
	assert.True(t, found.RestartRequired)
}

func TestGitLabValues_WiresRegistryStorageSecret(t *testing.T) {
	o := &Orchestrator{namespace: "devsecops"}

	values := o.gitlabExternalSharedServiceValues(nil)

	registry, ok := values["registry"].(map[string]any)
	require.True(t, ok, "registry 블록이 있어야 한다")

	storage, ok := registry["storage"].(map[string]any)
	require.True(t, ok, "registry.storage 가 있어야 한다")

	assert.Equal(t, ProvisionedRegistryStorageSecret, storage["secret"])
	assert.Equal(t, RegistryStorageSecretKey, storage["key"])
}

// 멀티라인 템플릿이 ExternalSecret 의 블록 스칼라로 들어가므로 들여쓰기가
// 깨지면 ESO 리소스 자체가 무효가 된다. 렌더 결과가 YAML 로 파싱되는지 고정한다.
func TestExternalSecretManifest_RegistryStorageIsValidYAML(t *testing.T) {
	var item *ManagedSecret
	for _, m := range managedSecrets("devsecops") {
		if m.TargetSecret == ProvisionedRegistryStorageSecret {
			m := m
			item = &m
			break
		}
	}
	require.NotNil(t, item)

	manifest := externalSecretManifest("devsecops", "kv/nullus/dev/org/", *item)

	var doc struct {
		Spec struct {
			Target struct {
				Template struct {
					Data map[string]string `yaml:"data"`
				} `yaml:"template"`
			} `yaml:"target"`
		} `yaml:"spec"`
	}
	require.NoError(t, yaml.Unmarshal([]byte(manifest), &doc), "렌더된 ExternalSecret 이 YAML 로 파싱되어야 한다")

	cfg := doc.Spec.Target.Template.Data[RegistryStorageSecretKey]
	require.NotEmpty(t, cfg, "config 키가 비어 있으면 registry 가 기본 filesystem 으로 돌아간다")

	// 블록 스칼라 안의 내용은 ESO 가 template 을 렌더링한 뒤에야 YAML 이 된다.
	// ({{ .accessKey }} 는 렌더 전에는 YAML 맵으로 잘못 읽힌다)
	rendered := strings.NewReplacer(
		"{{ .accessKey }}", "nullus-admin",
		"{{ .secretKey }}", "s3cr3t",
	).Replace(cfg)

	var storage struct {
		S3 struct {
			Bucket         string `yaml:"bucket"`
			RegionEndpoint string `yaml:"regionendpoint"`
			PathStyle      bool   `yaml:"pathstyle"`
		} `yaml:"s3"`
	}
	require.NoError(t, yaml.Unmarshal([]byte(rendered), &storage))
	assert.Equal(t, RegistryStorageBucket, storage.S3.Bucket)
	assert.Equal(t, "http://nullus-minio.devsecops.svc.cluster.local:9000", storage.S3.RegionEndpoint)
	assert.True(t, storage.S3.PathStyle)
}

// 차트 기본 replica 2개 + filesystem 조합이 되살아나지 않도록,
// valuesForStep 을 통과한 최종 values 에서도 유지되는지 확인한다.
func TestValuesForStep_GitLabKeepsRegistryStorageSecret(t *testing.T) {
	o := &Orchestrator{namespace: "devsecops"}

	spec, ok := defaultChartSpecForStep("installing_gitlab")
	require.True(t, ok)

	values := o.valuesForStep("installing_gitlab", spec)

	registry, ok := values["registry"].(map[string]any)
	require.True(t, ok)
	storage, ok := registry["storage"].(map[string]any)
	require.True(t, ok, "DefaultValues 의 registry 블록과 병합되며 storage 가 사라지면 안 된다")
	assert.Equal(t, ProvisionedRegistryStorageSecret, storage["secret"])
}
