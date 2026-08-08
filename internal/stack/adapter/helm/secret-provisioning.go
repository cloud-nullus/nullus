package helm

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/cloud-nullus/draft/internal/shared/secrets"
	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// 시크릿 지도 — provisioning_secrets 스텝이 생성·저장하는 목록이다.
//
// 지금까지는 Helm values 에 비밀번호를 하드코딩하고 나중에 같은 문자열을
// OpenBao 에 기록했다. 그 방향을 뒤집는다.
//
//	Nullus 생성 → OpenBao write → ExternalSecret → K8s Secret → 차트가 existingSecret 으로 참조
//
// 차트마다 요구하는 Secret 키 이름이 다르므로 하나의 대상 Secret 에 여러
// 항목을 담을 수 있어야 한다.

// SecretEntry 는 대상 Secret 안의 키 하나에 대응한다.
type SecretEntry struct {
	// PathSuffix 는 kv/nullus/{env}/{org}/ 뒤에 붙는 OpenBao 경로다.
	PathSuffix string
	// TargetKey 는 Kubernetes Secret 안의 키 이름이다. 차트가 요구하는 이름과
	// 정확히 일치해야 한다.
	TargetKey string
	// Fixed 가 비어 있지 않으면 랜덤 생성 대신 이 값을 쓴다.
	// 사용자명처럼 비밀이 아니지만 차트가 같은 Secret 안에서 요구하는 값에 쓴다.
	Fixed string
}

// ManagedSecret 은 ESO 가 소유하는 대상 Secret 하나를 기술한다.
//
// RestartRequired 는 회전 후 반영 전략의 스펙이다. 소비 방식에 따라 재시작
// 필요 여부가 달라진다 — Runner 는 기동 시 config 를 1회만 읽지만 ArgoCD 의
// repository Secret 은 매 요청 시점에 읽는다.
type ManagedSecret struct {
	TargetSecret    string
	Consumer        string
	RestartRequired bool
	Entries         []SecretEntry
	// TemplateData 가 있으면 ExternalSecret 의 target.template 으로 렌더링한다.
	// 값 안에서 {{ .키 }} 로 Entries 의 TargetKey 를 참조한다.
	TemplateData map[string]string
}

// 프로비저닝된 Secret 이름. 차트 values 가 existingSecret 으로 참조한다.
//
// 값의 단일 출처는 domain 이다 — 설치 경로와 조회 경로(연결정보 안내)가 같은
// 이름을 봐야 하는데, 양쪽에 각각 적어 두면 한쪽만 바뀌었을 때 조용히 어긋난다.
const (
	ProvisionedPostgresSecret      = domain.ProvisionedPostgresSecret
	ProvisionedMinIOSecret         = domain.ProvisionedMinIOSecret
	ProvisionedGitLabRootSecret    = "gitlab-initial-root-password" // #nosec G101 -- Secret 리소스 이름
	ProvisionedObjectStorageSecret = domain.ProvisionedObjectStorageSecret

	// Container Registry 전용 스토리지 설정. Rails 의 object_store 와 형식이
	// 달라(Docker distribution 스키마) 같은 Secret 에 담을 수 없다.
	ProvisionedRegistryStorageSecret = domain.ProvisionedRegistryStorageSecret
	RegistryStorageSecretKey         = "config"
	RegistryStorageBucket            = "gitlab-registry"

	// MinIORootUser 는 비밀이 아니지만 차트의 existingSecret 이 같은 Secret 안에서
	// 요구하므로 함께 프로비저닝한다.
	MinIORootUser = domain.MinIORootUser
)

// managedSecrets 는 현재 관리 대상 시크릿이다.
//
// provider 발급이 필요한 토큰(Runner 등록 토큰, registry PAT)은 해당 OSS 설치
// 이후에만 얻을 수 있으므로 이 단계가 아니라 회전 컨트롤러가 담당한다.
func managedSecrets(namespace string) []ManagedSecret {
	if strings.TrimSpace(namespace) == "" {
		namespace = defaultStackNamespace
	}
	minioEndpoint := fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", domain.MinIOServiceName, namespace, domain.MinIOServicePort)

	return []ManagedSecret{
		{
			// bitnami postgresql 차트는 auth.existingSecret 의 키 이름을
			// auth.secretKeys 로 지정한다.
			TargetSecret:    ProvisionedPostgresSecret,
			Consumer:        "PostgreSQL, GitLab",
			RestartRequired: true,
			Entries: []SecretEntry{
				{PathSuffix: "storage/postgresql/password", TargetKey: "password"},
				{PathSuffix: "storage/postgresql/admin-password", TargetKey: "postgres-password"},
				{PathSuffix: "storage/postgresql/replication-password", TargetKey: "replication-password"},
			},
		},
		{
			// charts.min.io 의 existingSecret 은 rootUser/rootPassword 키를 읽는다.
			TargetSecret:    ProvisionedMinIOSecret,
			Consumer:        "MinIO, GitLab(object storage)",
			RestartRequired: true,
			Entries: []SecretEntry{
				{PathSuffix: "artifacts/minio/root-user", TargetKey: domain.MinIOUserKey, Fixed: MinIORootUser},
				{PathSuffix: "artifacts/minio/root-password", TargetKey: domain.MinIOPasswordKey},
			},
		},
		{
			// GitLab 차트의 global.initialRootPassword 가 참조한다.
			TargetSecret:    ProvisionedGitLabRootSecret,
			Consumer:        "GitLab",
			RestartRequired: true,
			Entries: []SecretEntry{
				{PathSuffix: "artifacts/gitlab/root-password", TargetKey: "password"},
			},
		},
		{
			// GitLab object storage 연결 정보. 값 자체는 MinIO 자격이므로
			// 같은 OpenBao 경로를 참조하고 ESO template 으로 YAML 을 만든다.
			// 하드코딩 연결 문자열을 대체한다.
			TargetSecret:    ProvisionedObjectStorageSecret,
			Consumer:        "GitLab(object storage)",
			RestartRequired: true,
			Entries: []SecretEntry{
				{PathSuffix: "artifacts/minio/root-user", TargetKey: "accessKey", Fixed: MinIORootUser},
				{PathSuffix: "artifacts/minio/root-password", TargetKey: "secretKey"},
			},
			TemplateData: map[string]string{
				"connection": objectStorageConnectionTemplate(minioEndpoint),
				"config":     objectStorageConnectionTemplate(minioEndpoint),
			},
		},
		{
			// Container Registry 스토리지. 차트 기본값은 filesystem(/tmp/registry)
			// 이라 파드 재시작 시 이미지가 사라지고, replica 가 2개면 push 한 파드와
			// pull 하는 파드가 달라 비결정적으로 실패한다. S3(MinIO) 로 고정한다.
			TargetSecret:    ProvisionedRegistryStorageSecret,
			Consumer:        "GitLab(container registry)",
			RestartRequired: true,
			Entries: []SecretEntry{
				{PathSuffix: "artifacts/minio/root-user", TargetKey: "accessKey", Fixed: MinIORootUser},
				{PathSuffix: "artifacts/minio/root-password", TargetKey: "secretKey"},
			},
			TemplateData: map[string]string{
				RegistryStorageSecretKey: registryStorageConfigTemplate(minioEndpoint),
			},
		},
	}
}

// registryStorageConfigTemplate 은 Docker distribution 의 storage 블록이다.
//
// Rails 의 object_store(objectStorageConnectionTemplate)와 키 이름이 다르다 —
// registry 는 distribution 스키마(accesskey/secretkey/regionendpoint)를 쓰므로
// 같은 Secret 을 재사용할 수 없다.
func registryStorageConfigTemplate(endpoint string) string {
	return "s3:\n" +
		"  bucket: " + RegistryStorageBucket + "\n" +
		"  accesskey: {{ .accessKey }}\n" +
		"  secretkey: {{ .secretKey }}\n" +
		"  region: us-east-1\n" +
		"  regionendpoint: " + endpoint + "\n" +
		"  v4auth: true\n" +
		"  pathstyle: true\n" +
		// 리다이렉트를 끄면 레지스트리가 blob 을 직접 흘려보낸다.
		// 켜두면 클라이언트를 MinIO 의 클러스터 내부 주소로 보내는데,
		// kubelet 은 클러스터 DNS 를 쓰지 않아 그 주소를 해석하지 못하고
		// 이미지 pull 이 타임아웃으로 실패한다.
		"redirect:\n" +
		"  disable: true\n"
}

// objectStorageConnectionTemplate 은 ESO template 으로 렌더링할 연결 YAML 이다.
func objectStorageConnectionTemplate(endpoint string) string {
	return "provider: AWS\n" +
		"region: us-east-1\n" +
		"aws_access_key_id: {{ .accessKey }}\n" +
		"aws_secret_access_key: {{ .secretKey }}\n" +
		"endpoint: " + endpoint + "\n" +
		"path_style: true\n"
}

// generateSecretValue 는 32바이트 랜덤을 URL-safe base64 로 만든다.
func generateSecretValue() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("랜덤 생성 실패: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// externalSecretManifest 는 하나의 관리 시크릿에 대한 ExternalSecret 을 만든다.
//
// creationPolicy: Owner 로 ESO 가 대상 Secret 의 소유자가 된다. 차트는 이
// Secret 을 만들지 않고 existingSecret 으로 참조만 하므로 소유권이 충돌하지 않는다.
func externalSecretManifest(namespace, pathPrefix string, item ManagedSecret) string {
	var b strings.Builder
	fmt.Fprintf(&b, `apiVersion: external-secrets.io/v1
kind: ExternalSecret
metadata:
  name: %s
  namespace: %s
spec:
  refreshInterval: 5m
  secretStoreRef:
    name: %s
    kind: SecretStore
  target:
    name: %s
    creationPolicy: Owner
`, item.TargetSecret, namespace, ESOSecretStoreName, item.TargetSecret)

	if len(item.TemplateData) > 0 {
		b.WriteString("    template:\n      engineVersion: v2\n      data:\n")
		// 결정적 순서를 위해 키를 정렬한다.
		keys := make([]string, 0, len(item.TemplateData))
		for k := range item.TemplateData {
			keys = append(keys, k)
		}
		sortStrings(keys)
		for _, k := range keys {
			fmt.Fprintf(&b, "        %s: |\n%s\n", k, indentYAML(item.TemplateData[k], 10))
		}
	}

	b.WriteString("  data:\n")
	for _, entry := range item.Entries {
		fmt.Fprintf(&b, `    - secretKey: %s
      remoteRef:
        key: %s
        property: token
`, entry.TargetKey, pathPrefix+entry.PathSuffix)
	}
	return b.String()
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// secretPathPrefix 는 이 스택의 OpenBao 경로 접두사다.
// 경로 규약: kv/nullus/{env}/{org_id}/
func secretPathPrefix(env, orgID string) string {
	env = strings.TrimSpace(env)
	if env == "" {
		env = "dev"
	}
	orgID = strings.TrimSpace(orgID)
	if orgID == "" {
		orgID = "default"
	}
	return fmt.Sprintf("%s/nullus/%s/%s/", OpenBaoKVMount, env, orgID)
}

// SecretWriter 는 OpenBao 에 값을 기록하고 읽는 최소 인터페이스다.
type SecretWriter interface {
	PutToken(ctx context.Context, path, value string) error
	GetToken(ctx context.Context, path string) (string, error)
}

// ProvisionSecrets 는 시크릿을 생성해 OpenBao 에 기록하고 ExternalSecret 을
// 적용한 뒤, ESO 가 실제 Kubernetes Secret 을 만들 때까지 기다린다.
//
// 대기가 필수다. ExternalSecret 을 apply 해도 ESO 가 Secret 을 만들기까지
// 시간이 걸리고, 그 전에 후속 Helm 설치가 existingSecret 을 참조하면
// 파드가 기동에 실패한다.
//
// 이미 값이 있으면 보존한다. 재설치·스텝 재시도로 비밀번호가 바뀌면 기존
// 데이터에 접근할 수 없게 되기 때문이다.
func (o *Orchestrator) ProvisionSecrets(
	ctx context.Context,
	namespace, env, orgID string,
	writer SecretWriter,
) error {
	if !looksLikeKubeconfig(o.kubeconfig) {
		return nil
	}
	if strings.TrimSpace(namespace) == "" {
		namespace = o.namespace
	}
	if writer == nil {
		return fmt.Errorf("secret writer 가 구성되지 않았습니다")
	}

	prefix := secretPathPrefix(env, orgID)

	// 기본 시크릿에 SSO client secret 을 더한다.
	// 설치되는 도구가 스택마다 다르므로 목록도 스택 구성에서 파생된다.
	items := append(managedSecrets(namespace), o.ssoManagedSecrets()...)

	for _, item := range items {
		for _, entry := range item.Entries {
			path := prefix + entry.PathSuffix
			if err := ensureSecretValue(ctx, writer, path, entry.Fixed); err != nil {
				return err
			}
		}
		if err := o.applyManifest(ctx, namespace, externalSecretManifest(namespace, prefix, item)); err != nil {
			return fmt.Errorf("ExternalSecret 적용 실패 (%s): %w", item.TargetSecret, err)
		}
	}

	return o.waitForProvisionedSecrets(ctx, namespace, items)
}

// ensureSecretValue 는 값이 없을 때만 생성해 기록한다 (멱등).
func ensureSecretValue(ctx context.Context, writer SecretWriter, path, fixed string) error {
	if existing, err := writer.GetToken(ctx, path); err == nil && strings.TrimSpace(existing) != "" {
		return nil
	}
	value := fixed
	if value == "" {
		generated, err := generateSecretValue()
		if err != nil {
			return err
		}
		value = generated
	}
	if err := writer.PutToken(ctx, path, value); err != nil {
		return fmt.Errorf("OpenBao 기록 실패 (%s): %w", path, err)
	}
	return nil
}

// waitForProvisionedSecrets 는 ESO 가 대상 Secret 을 모두 만들 때까지 기다린다.
func (o *Orchestrator) waitForProvisionedSecrets(ctx context.Context, namespace string, items []ManagedSecret) error {
	const (
		maxAttempts = 36
		retryDelay  = 5 * time.Second
	)

	for _, item := range items {
		created := false
		for attempt := 1; attempt <= maxAttempts; attempt++ {
			if _, err := o.runKubectl(ctx, "get", "secret", item.TargetSecret, "-n", namespace); err == nil {
				created = true
				break
			}
			if attempt == maxAttempts {
				break
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(retryDelay):
			}
		}
		if !created {
			return fmt.Errorf("ESO 가 Secret %q 를 생성하지 못했습니다", item.TargetSecret)
		}
	}
	return nil
}

// SetSecretScope 는 OpenBao 경로 접두사에 쓰이는 환경/조직을 설정한다.
// install_stack 이 스택 컨텍스트를 알고 있으므로 거기서 주입한다.
func (o *Orchestrator) SetSecretScope(env, orgID string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.secretEnv = strings.TrimSpace(env)
	o.secretOrgID = strings.TrimSpace(orgID)
}

// runSecretProvisioning 은 provisioning_secrets 스텝의 실행 본체다.
//
// 쓰기 권한이 필요하므로 컨트롤러 role 로 Kubernetes Auth 로그인한 store 를
// 사용한다. 정적 토큰은 쓰지 않는다.
func (o *Orchestrator) runSecretProvisioning(ctx context.Context, namespace string) error {
	if !looksLikeKubeconfig(o.kubeconfig) {
		return nil
	}
	if strings.TrimSpace(namespace) == "" {
		namespace = o.namespace
	}

	o.mu.Lock()
	env, orgID := o.secretEnv, o.secretOrgID
	o.mu.Unlock()

	store, err := secrets.NewKubernetesAuthStore(secrets.KubernetesAuthConfig{
		Kubeconfig:     o.kubeconfig,
		Namespace:      namespace,
		Role:           secrets.ControllerRole,
		ServiceAccount: secrets.ControllerServiceAccount,
	})
	if err != nil {
		return fmt.Errorf("OpenBao 컨트롤러 자격 생성 실패: %w", err)
	}

	return o.ProvisionSecrets(ctx, namespace, env, orgID, store)
}
