package helm

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"strings"

	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
	"github.com/cloud-nullus/draft/internal/stack/domain"
)

const objectStorageBucketJobName = "nullus-gitlab-bucket-bootstrap"

var gitLabRequiredBuckets = []string{
	// 연결정보가 오브젝트 스토리지의 resource_name 으로 안내하는 버킷이다.
	domain.GitLabArtifactsBucket,
	"git-lfs",
	"gitlab-uploads",
	"gitlab-packages",
	"gitlab-pages",
	// Container Registry 전용. 나머지 버킷은 Rails 의 object_store 가 쓰지만
	// registry 는 Docker distribution 이 직접 S3 를 붙으므로 분리한다.
	RegistryStorageBucket,
}

func (o *Orchestrator) currentStackConfig() *domain.StackConfig {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.stackConfig == nil {
		return nil
	}
	cfg := *o.stackConfig
	return &cfg
}

// objectStorageBackend 는 이 스택이 고른 오브젝트 스토리지 구현체다.
// 설정을 모르면 MinIO 다(domain.ObjectStorageBackendFor 의 기본값과 같다).
func (o *Orchestrator) objectStorageBackend() domain.ObjectStorageBackend {
	cfg := o.currentStackConfig()
	if cfg == nil {
		return domain.MinIOObjectStorage
	}
	if backend, ok := domain.ObjectStorageBackendFor(cfg.Artifacts.StorageBackend); ok {
		return backend
	}
	return domain.MinIOObjectStorage
}

// ensureObjectStorageBackendNotSwitched 는 이미 다른 구현체로 선 스택에서 구현체를 바꾸지
// 못하게 막는다.
//
// 바꾸면 GitLab 의 연결 Secret 이 빈 새 스토리지를 가리키게 되어 기존 아티팩트·업로드·
// 이미지가 보이지 않고, 옛 릴리스는 스택을 지울 때까지 고아로 남는다. 오브젝트를 옮기는
// 기능은 없다. 확인 자체가 실패하면(클러스터 오류) 막지 않는다 — 설치가 그 자리에서
// 진짜 이유로 실패한다.
func (o *Orchestrator) ensureObjectStorageBackendNotSwitched(ctx context.Context, selected domain.ObjectStorageBackend, namespace string) error {
	if o.installer == nil {
		return nil
	}
	for _, other := range domain.ObjectStorageBackends() {
		if other.Tool == selected.Tool {
			continue
		}
		_, err := o.installer.Status(ctx, other.ReleaseName, namespace)
		switch {
		case err == nil:
			return fmt.Errorf("이 스택에는 이미 %s(릴리스 %s)가 서 있어 %s 로 바꿀 수 없습니다 — 오브젝트를 옮기는 기능이 없어 GitLab 이 기존 아티팩트·업로드·이미지를 잃습니다. storage_backend 를 %s 로 되돌리거나 스택을 새로 만드십시오",
				other.DisplayName, other.ReleaseName, selected.DisplayName, other.DisplayName)
		case isReleaseNotFoundError(err):
			continue
		default:
			slog.Warn("다른 오브젝트 스토리지 릴리스를 확인하지 못했습니다", "release", other.ReleaseName, "namespace", namespace, "error", err)
		}
	}
	return nil
}

// guardObjectStorageSwitch 는 이 스택이 오브젝트 스토리지를 고른 경우에만 전환 가드를 돈다.
// 실클러스터가 아니면(kubeconfig 가 아니면) 확인할 곳이 없다.
func (o *Orchestrator) guardObjectStorageSwitch(ctx context.Context) error {
	cfg := o.currentStackConfig()
	if cfg == nil || !looksLikeKubeconfig(o.kubeconfig) {
		return nil
	}
	backend, ok := domain.ObjectStorageBackendFor(cfg.Artifacts.StorageBackend)
	if !ok {
		return nil
	}
	return o.ensureObjectStorageBackendNotSwitched(ctx, backend, o.namespace)
}

// ensureGitLabObjectStorageBuckets 는 GitLab 이 쓸 버킷을 만든다.
//
// 벤더 CLI(mc) 를 쓰지 않는다. S3 의 CreateBucket 은 서명한 PUT 하나라 curl 의
// --aws-sigv4 로 부르면 MinIO·SeaweedFS·외부 S3 어디서나 같다 — 구현체를 갈아
// 끼워도 이 Job 은 바뀌지 않는다.
//
// 자격은 매니페스트에 평문으로 싣지 않는다. 같은 네임스페이스의 Secret 을
// 환경변수로 읽는다(objectStorageTarget.CredentialSecret).
func (o *Orchestrator) ensureGitLabObjectStorageBuckets(ctx context.Context, namespace string) error {
	target, err := o.resolveObjectStorageTarget(ctx, namespace)
	if err != nil {
		return err
	}

	manifest := bucketBootstrapJobManifest(namespace, target, gitLabRequiredBuckets)

	_, _ = o.runKubectl(ctx, "delete", "job", objectStorageBucketJobName, "-n", namespace, "--ignore-not-found=true")
	if err := o.applyManifest(ctx, namespace, manifest); err != nil {
		return err
	}
	if _, err := o.runKubectl(ctx, "wait", "-n", namespace, "--for=condition=complete", "--timeout=180s", "job/"+objectStorageBucketJobName); err != nil {
		logs, _ := o.runKubectl(ctx, "logs", "-n", namespace, "job/"+objectStorageBucketJobName)
		return fmt.Errorf("bucket bootstrap job failed: %w (%s)", err, strings.TrimSpace(string(logs)))
	}
	if _, err := o.runKubectl(ctx, "delete", "job", objectStorageBucketJobName, "-n", namespace, "--ignore-not-found=true"); err != nil {
		return fmt.Errorf("cleanup bucket bootstrap job: %w", err)
	}

	return nil
}

type objectStorageTarget struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	// CredentialSecret 이 있으면 Job 이 자격을 이 Secret 에서 읽는다(AccessKeyKey·
	// SecretKeyKey). 없으면 AccessKey·SecretKey 를 평문 환경변수로 싣는다 — 외부
	// 스토리지를 직접 입력한 경우처럼 클러스터에 그 값을 담은 Secret 이 없을 때다.
	CredentialSecret string
	AccessKeyKey     string
	SecretKeyKey     string
}

func (o *Orchestrator) resolveObjectStorageTarget(ctx context.Context, namespace string) (objectStorageTarget, error) {
	cfg := o.currentStackConfig()
	if cfg == nil || cfg.Storage == nil {
		return o.internalObjectStorageTarget(ctx, namespace)
	}

	target := cfg.Storage.ObjectStorage
	if strings.TrimSpace(target.Mode) != "existing-connect" {
		return o.internalObjectStorageTarget(ctx, namespace)
	}

	endpoint := strings.TrimSpace(target.Endpoint)
	if endpoint == "" {
		return objectStorageTarget{}, fmt.Errorf("storage.object_storage.endpoint is required for existing-connect")
	}

	accessKey := strings.TrimSpace(target.AuthID)
	secretKey := strings.TrimSpace(target.AuthPasswordKey)
	if strings.TrimSpace(target.AccessSecretRef) != "" {
		resolvedAccessKey, resolvedSecretKey, err := o.resolveObjectStorageCredentialsFromSecret(ctx, namespace, target)
		if err != nil {
			return objectStorageTarget{}, err
		}
		if strings.TrimSpace(resolvedAccessKey) != "" {
			accessKey = resolvedAccessKey
		}
		if strings.TrimSpace(resolvedSecretKey) != "" {
			secretKey = resolvedSecretKey
		}
	}

	if accessKey == "" || secretKey == "" {
		return objectStorageTarget{}, fmt.Errorf("storage.object_storage credentials could not be resolved for existing-connect")
	}

	return objectStorageTarget{Endpoint: endpoint, AccessKey: accessKey, SecretKey: secretKey}, nil
}

func (o *Orchestrator) resolveObjectStorageCredentialsFromSecret(ctx context.Context, namespace string, target domain.StorageTarget) (string, string, error) {
	secretRef := strings.TrimSpace(target.AccessSecretRef)
	if secretRef == "" {
		return "", "", fmt.Errorf("storage.object_storage.access_secret_ref is empty")
	}
	passwordKeyRef := strings.TrimSpace(target.AuthPasswordKey)
	if passwordKeyRef == "" {
		return "", "", fmt.Errorf("storage.object_storage.auth_password_key is empty")
	}

	accessKeyRef := strings.TrimSpace(target.AuthID)
	if accessKeyRef == "" {
		accessKeyRef = "accessKey"
	}

	accessKey, accessErr := o.getSecretValue(ctx, namespace, secretRef, accessKeyRef)
	if accessErr != nil {
		accessKey = strings.TrimSpace(target.AuthID)
	}
	secretKey, secretErr := o.getSecretValue(ctx, namespace, secretRef, passwordKeyRef)
	if secretErr != nil {
		secretKey = strings.TrimSpace(target.AuthPasswordKey)
	}
	if strings.TrimSpace(accessKey) == "" || strings.TrimSpace(secretKey) == "" {
		return "", "", fmt.Errorf("failed to resolve object storage credentials from secret %q (access=%q secret=%q)", secretRef, accessKeyRef, passwordKeyRef)
	}

	return strings.TrimSpace(accessKey), strings.TrimSpace(secretKey), nil
}

func (o *Orchestrator) getSecretValue(ctx context.Context, namespace, name, key string) (string, error) {
	jsonpath := fmt.Sprintf("jsonpath={.data.%s}", escapeJSONPathKey(key))
	out, err := o.runKubectl(ctx, "get", "secret", name, "-n", namespace, "-o", jsonpath)
	if err != nil {
		return "", err
	}
	encoded := strings.TrimSpace(string(out))
	if encoded == "" {
		return "", fmt.Errorf("key %q missing in secret %q", key, name)
	}
	decoded, err := decodeBase64String(encoded)
	if err != nil {
		return "", fmt.Errorf("decode key %q from secret %q: %w", key, name, err)
	}
	return decoded, nil
}

func escapeJSONPathKey(key string) string {
	replacer := strings.NewReplacer(".", "\\.", "-", "\\-")
	return replacer.Replace(key)
}

// bucketBootstrapJobManifest 는 버킷을 만드는 Job 이다.
func bucketBootstrapJobManifest(namespace string, target objectStorageTarget, buckets []string) string {
	env := ""
	if strings.TrimSpace(target.CredentialSecret) != "" {
		env = fmt.Sprintf(`        - name: S3_ACCESS_KEY
          valueFrom:
            secretKeyRef:
              name: %s
              key: %s
        - name: S3_SECRET_KEY
          valueFrom:
            secretKeyRef:
              name: %s
              key: %s
`, target.CredentialSecret, target.AccessKeyKey, target.CredentialSecret, target.SecretKeyKey)
	} else {
		env = fmt.Sprintf(`        - name: S3_ACCESS_KEY
          value: %q
        - name: S3_SECRET_KEY
          value: %q
`, target.AccessKey, target.SecretKey)
	}
	return fmt.Sprintf(`apiVersion: batch/v1
kind: Job
metadata:
  name: %s
  namespace: %s
spec:
  backoffLimit: 0
  template:
    spec:
      restartPolicy: Never
      containers:
      - name: s3
        image: %s
        env:
        - name: S3_ENDPOINT
          value: %q
%s        command: ["/bin/sh", "-c"]
        args:
          - |
%s
`, objectStorageBucketJobName, namespace, shareddomain.CurlImage, s3EndpointURL(target.Endpoint), env,
		indentYAML(buildBucketBootstrapScript(buckets), 12))
}

// s3EndpointURL 은 curl 이 부를 수 있는 URL 로 맞춘다. 외부 스토리지를 입력할 때
// 스킴을 빼는 경우가 있다(host:port) — 클러스터 안 주소처럼 http 로 본다.
func s3EndpointURL(endpoint string) string {
	endpoint = strings.TrimRight(strings.TrimSpace(endpoint), "/")
	if !strings.Contains(endpoint, "://") {
		endpoint = "http://" + endpoint
	}
	return endpoint
}

// buildBucketBootstrapScript 는 S3 로 버킷을 확인하고 없으면 만드는 스크립트다.
//
// 먼저 HEAD 로 본다. 200 이면 만들지 않는다 — 외부 S3(다른 리전의 AWS 등)는 이미 있는
// 버킷에 대한 PUT 을 리전·본문 문제로 거절할 수 있어, 버킷이 있는데도 설치가 멈춘다.
// 404 일 때만 만든다(PUT, 200 · 409 는 성공 — 그사이 다른 쪽이 만들었을 수 있다). 그 뒤
// HEAD 가 200 이어야 끝난다: 만들기를 받아들이고도 그 키로 버킷을 읽지 못하면(권한·서명
// 불일치) GitLab 이 첫 업로드에서 죽는데, 여기서 잡는 편이 원인이 가깝다.
//
// 엔드포인트·자격은 환경변수로 받는다. 스크립트 안에 값이 없으므로 따옴표를
// 맞출 일도, 로그에 비밀이 찍힐 일도 없다.
//
// 스토리지 설치는 준비를 기다리지 않고 넘어오므로 먼저 엔드포인트가 응답할 때까지
// 기다린다(기본 60회 × 2초 — 단계가 Job 을 기다리는 180초 안쪽이다). 2026-10-19 kind
// 실측에서 SeaweedFS 게이트웨이가 포트를 열기 전에 만들기가 연결 거절로 죽었다 —
// mc 는 SDK 가 알아서 재시도해 이 경합이 가려져 있었다. 뜬 뒤의 일시적 5xx 는 curl 의
// --retry 가 받는다(409·403 같은 확정 응답은 다시 보내지 않는다).
//
// 모든 호출에 시간 제한을 둔다. 패킷을 버리는 엔드포인트(NetworkPolicy·방화벽)에서는
// 연결 시도 하나가 SYN 재전송으로 2분 넘게 붙잡혀 대기 횟수가 아무것도 막지 못하고,
// 단계는 이유 없는 시간 초과로 죽는다.
func buildBucketBootstrapScript(buckets []string) string {
	var sb strings.Builder
	sb.WriteString("set -eu\n")
	sb.WriteString(fmt.Sprintf("SIG='aws:amz:%s:s3'\n", domain.ObjectStorageRegion))
	sb.WriteString("i=0\n")
	sb.WriteString("until [ \"$(curl -s --connect-timeout 3 --max-time 10 -o /dev/null -w '%{http_code}' \"$S3_ENDPOINT/\" || true)\" != 000 ]; do\n")
	sb.WriteString("  i=$((i+1))\n")
	sb.WriteString("  if [ \"$i\" -ge \"${S3_WAIT_ATTEMPTS:-60}\" ]; then echo \"S3 endpoint not reachable: $S3_ENDPOINT\" >&2; exit 1; fi\n")
	sb.WriteString("  sleep \"${S3_WAIT_DELAY:-2}\"\n")
	sb.WriteString("done\n")
	sb.WriteString("s3() { curl -sS --connect-timeout 5 --max-time 30 --retry 5 --retry-delay 2 -o /dev/null -w '%{http_code}' --aws-sigv4 \"$SIG\" --user \"$S3_ACCESS_KEY:$S3_SECRET_KEY\" \"$@\"; }\n")
	for _, bucket := range buckets {
		sb.WriteString(fmt.Sprintf("code=$(s3 -I \"$S3_ENDPOINT/%s\")\n", bucket))
		sb.WriteString("if [ \"$code\" = 404 ]; then\n")
		sb.WriteString(fmt.Sprintf("  code=$(s3 -X PUT -H 'Content-Length: 0' \"$S3_ENDPOINT/%s\")\n", bucket))
		sb.WriteString(fmt.Sprintf("  case \"$code\" in 200|409) ;; *) echo \"create bucket %s: HTTP $code\" >&2; exit 1;; esac\n", bucket))
		sb.WriteString(fmt.Sprintf("  code=$(s3 -I \"$S3_ENDPOINT/%s\")\n", bucket))
		sb.WriteString("fi\n")
		sb.WriteString(fmt.Sprintf("[ \"$code\" = 200 ] || { echo \"head bucket %s: HTTP $code\" >&2; exit 1; }\n", bucket))
		sb.WriteString(fmt.Sprintf("echo \"bucket %s ready\"\n", bucket))
	}
	return sb.String()
}

func decodeBase64String(value string) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}

// internalObjectStorageTarget 은 스택 안에 세운 오브젝트 스토리지 접속 정보다.
//
// 어느 구현체든 같다 — 엔드포인트와 자격 Secret 의 위치만 구현체 표에서 온다.
// 자격증명은 하드코딩하지 않고 프로비저닝된 Secret 에서 읽는다. ESO 가 그
// Secret 의 소유자이며 원천은 OpenBao 다.
func (o *Orchestrator) internalObjectStorageTarget(ctx context.Context, namespace string) (objectStorageTarget, error) {
	backend := o.objectStorageBackend()
	target := objectStorageTarget{
		Endpoint:         backend.Endpoint(namespace),
		AccessKey:        domain.ObjectStorageAccessKey,
		CredentialSecret: backend.CredentialSecret,
		AccessKeyKey:     backend.AccessKeyKey,
		SecretKeyKey:     backend.SecretKeyKey,
	}

	// Job 은 Secret 을 직접 참조하지만, 그 Secret 이 아직 없으면 파드가
	// CreateContainerConfigError 로 서서 180초 대기 끝에야 실패한다. 여기서 먼저
	// 읽어 보아 이유를 바로 돌려준다.
	secretKey, err := o.readSecretValue(ctx, namespace, backend.CredentialSecret, backend.SecretKeyKey)
	if err != nil {
		return objectStorageTarget{}, fmt.Errorf("%s 자격증명을 읽지 못했습니다 (%s): %w", backend.DisplayName, backend.CredentialSecret, err)
	}
	target.SecretKey = secretKey

	if user, err := o.readSecretValue(ctx, namespace, backend.CredentialSecret, backend.AccessKeyKey); err == nil && strings.TrimSpace(user) != "" {
		target.AccessKey = user
	}
	return target, nil
}

// readSecretValue 는 Kubernetes Secret 의 값 하나를 디코드해 돌려준다.
func (o *Orchestrator) readSecretValue(ctx context.Context, namespace, name, key string) (string, error) {
	out, err := o.runKubectl(ctx, "get", "secret", name, "-n", namespace,
		"-o", fmt.Sprintf("jsonpath={.data.%s}", key))
	if err != nil {
		return "", err
	}
	encoded := strings.TrimSpace(string(out))
	if encoded == "" {
		return "", fmt.Errorf("secret %s 에 %s 키가 없습니다", name, key)
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("secret %s/%s 디코드 실패: %w", name, key, err)
	}
	return string(decoded), nil
}
