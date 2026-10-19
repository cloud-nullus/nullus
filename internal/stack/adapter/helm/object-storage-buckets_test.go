package helm

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// 버킷은 S3 CreateBucket 을 HTTP 로 직접 부른다 — 벤더 CLI(mc) 에 묶이면 그
// 구현체가 막히는 날 다른 구현체로도 버킷을 만들 수 없다.
func TestBuildBucketBootstrapScript_CallsS3OverHTTPOnly(t *testing.T) {
	script := buildBucketBootstrapScript([]string{"a", "b"})

	assert.NotContains(t, script, "mc ")
	assert.Contains(t, script, "--aws-sigv4")
	assert.Contains(t, script, "aws:amz:"+domain.ObjectStorageRegion+":s3")
	for _, bucket := range []string{"a", "b"} {
		// 본문 없는 PUT 은 Content-Length 를 싣지 않아 일부 S3 가 411 로 거절한다.
		assert.Contains(t, script, `-X PUT -H 'Content-Length: 0' "$S3_ENDPOINT/`+bucket+`"`)
		assert.Contains(t, script, `-I "$S3_ENDPOINT/`+bucket+`"`)
	}
	// 자격은 환경변수로만 들어온다. 스크립트에 값이 박히면 로그에 비밀이 찍힌다.
	assert.Contains(t, script, `$S3_ACCESS_KEY:$S3_SECRET_KEY`)
}

// 스크립트를 실제로 돌려 본다. curl 자리에 가짜를 두고 버킷 하나의 상태를 흉내 낸다.
//
//   - 서버가 아직 안 뜬 것처럼 처음 PROBE_FAILS 번의 호출은 종류와 무관하게 연결 거절(000, exit 7)
//   - 확인(-I): 버킷이 있으면 HEAD_CODE(기본 200), 없으면 MISSING_CODE(기본 404)
//   - 만들기(-X PUT): CREATE_CODE 를 돌려주고 200·409 면 버킷이 생긴다. 호출되면 PUT_CALLED 를 남긴다
//   - 그 밖(대기): 403 — S3 루트는 익명 요청을 거절한다
type bucketScriptCase struct {
	createCode, headCode, missingCode string
	probeFails                        int
	exists                            bool
	env                               []string
}

func runBucketScript(t *testing.T, c bucketScriptCase) (out string, putCalled bool, err error) {
	t.Helper()
	if _, lookErr := exec.LookPath("sh"); lookErr != nil {
		t.Skip("sh 가 없다")
	}
	dir := t.TempDir()
	state := filepath.Join(dir, "state")
	if c.exists {
		require.NoError(t, os.WriteFile(filepath.Join(state+".bucket"), nil, 0o644))
	}
	fake := "#!/bin/sh\n" +
		"n=$(cat \"$STATE.count\" 2>/dev/null || echo 0); n=$((n+1)); echo $n > \"$STATE.count\"\n" +
		"if [ $n -le \"$PROBE_FAILS\" ]; then printf '000'; exit 7; fi\n" +
		"for a in \"$@\"; do\n" +
		"  if [ \"$a\" = -I ]; then [ -e \"$STATE.bucket\" ] && printf '%s' \"$HEAD_CODE\" || printf '%s' \"$MISSING_CODE\"; exit 0; fi\n" +
		"  if [ \"$a\" = -X ]; then touch \"$STATE.put\"; case \"$CREATE_CODE\" in 200|409) touch \"$STATE.bucket\";; esac; printf '%s' \"$CREATE_CODE\"; exit 0; fi\n" +
		"done\n" +
		"printf '403'\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "curl"), []byte(fake), 0o755))
	or := func(v, fallback string) string {
		if v == "" {
			return fallback
		}
		return v
	}
	cmd := exec.Command("sh", "-c", buildBucketBootstrapScript([]string{"gitlab-artifacts"}))
	cmd.Env = append(os.Environ(),
		"PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"S3_ENDPOINT=http://s3.local:8333", "S3_ACCESS_KEY=ak", "S3_SECRET_KEY=sk",
		"CREATE_CODE="+or(c.createCode, "200"), "HEAD_CODE="+or(c.headCode, "200"),
		"MISSING_CODE="+or(c.missingCode, "404"),
		"PROBE_FAILS="+strconv.Itoa(c.probeFails), "STATE="+state, "S3_WAIT_DELAY=0")
	cmd.Env = append(cmd.Env, c.env...)
	raw, err := cmd.CombinedOutput()
	_, statErr := os.Stat(state + ".put")
	return string(raw), statErr == nil, err
}

func TestBuildBucketBootstrapScript_CreatesMissingBucket(t *testing.T) {
	out, put, err := runBucketScript(t, bucketScriptCase{})
	require.NoError(t, err, out)
	assert.True(t, put)
	assert.Contains(t, out, "bucket gitlab-artifacts ready")

	// 확인과 만들기 사이에 다른 쪽이 먼저 만든 경우(409)도 성공이다.
	out, _, err = runBucketScript(t, bucketScriptCase{createCode: "409"})
	require.NoError(t, err, out)
}

// 이미 있는 버킷에는 만들기를 보내지 않는다. 외부 S3(AWS 의 다른 리전 등)는 이미 있는
// 버킷에 대한 PUT 을 리전·본문 문제로 거절할 수 있어, 버킷이 있는데도 설치가 멈춘다.
func TestBuildBucketBootstrapScript_SkipsCreateForExistingBucket(t *testing.T) {
	out, put, err := runBucketScript(t, bucketScriptCase{exists: true, createCode: "400"})
	require.NoError(t, err, out)
	assert.False(t, put, "있는 버킷에 PUT 을 보냈다")
}

func TestBuildBucketBootstrapScript_FailsOnDeniedOrUnreadableBucket(t *testing.T) {
	out, _, err := runBucketScript(t, bucketScriptCase{createCode: "403"})
	require.Error(t, err, "권한 거절을 성공으로 넘기면 GitLab 이 첫 업로드에서 죽는다")
	assert.Contains(t, out, "create bucket gitlab-artifacts: HTTP 403")

	out, _, err = runBucketScript(t, bucketScriptCase{missingCode: "403"})
	require.Error(t, err, "확인부터 거절되면 그 자격으로는 버킷을 쓸 수 없다")
	assert.Contains(t, out, "head bucket gitlab-artifacts: HTTP 403")

	out, _, err = runBucketScript(t, bucketScriptCase{headCode: "404"})
	require.Error(t, err, "만들고도 읽지 못하면 실패여야 한다")
	assert.Contains(t, out, "head bucket gitlab-artifacts: HTTP 404")
}

// 스토리지 설치는 준비를 기다리지 않고 다음 단계로 넘어간다. 게이트웨이가 아직
// 포트를 열기 전에 버킷을 만들면 연결이 거절된다 — 2026-10-19 kind 실측에서
// SeaweedFS 를 고른 첫 설치가 이렇게 죽었다(mc 는 SDK 가 알아서 재시도해 가려졌다).
func TestBuildBucketBootstrapScript_WaitsForEndpoint(t *testing.T) {
	out, _, err := runBucketScript(t, bucketScriptCase{probeFails: 3})
	require.NoError(t, err, "엔드포인트가 늦게 떠도 기다렸다 만들어야 한다: %s", out)
	assert.Contains(t, out, "bucket gitlab-artifacts ready")

	out, _, err = runBucketScript(t, bucketScriptCase{probeFails: 99, env: []string{"S3_WAIT_ATTEMPTS=5"}})
	require.Error(t, err, "끝내 뜨지 않으면 이유를 남기고 실패해야 한다")
	assert.Contains(t, out, "S3 endpoint not reachable")
}

// 패킷을 버리는 엔드포인트(NetworkPolicy·방화벽)에서는 curl 이 SYN 재전송으로 2분 넘게
// 붙잡혀 대기 횟수가 아무것도 막지 못한다. 단계는 180초 뒤 빈 로그로 죽는다.
func TestBuildBucketBootstrapScript_BoundsEveryCall(t *testing.T) {
	script := buildBucketBootstrapScript([]string{"a"})
	for _, line := range strings.Split(script, "\n") {
		if strings.Contains(line, "curl ") {
			assert.Containsf(t, line, "--connect-timeout", "시간 제한 없는 curl: %s", line)
			assert.Containsf(t, line, "--max-time", "시간 제한 없는 curl: %s", line)
		}
	}
}

// 스택 안 스토리지는 자격을 Secret 참조로 넘긴다. 매니페스트(=Job 스펙)에 평문이
// 남지 않아야 한다 — Job 은 kubectl get 으로 누구나 읽는다.
func TestBucketBootstrapJobManifest_ReadsCredentialsFromSecret(t *testing.T) {
	target := objectStorageTarget{
		Endpoint:         domain.SeaweedFSObjectStorage.Endpoint("team-a"),
		AccessKey:        "nullus-admin",
		SecretKey:        "super-secret-value",
		CredentialSecret: domain.SeaweedFSObjectStorage.CredentialSecret,
		AccessKeyKey:     domain.SeaweedFSObjectStorage.AccessKeyKey,
		SecretKeyKey:     domain.SeaweedFSObjectStorage.SecretKeyKey,
	}
	manifest := bucketBootstrapJobManifest("team-a", target, gitLabRequiredBuckets)

	assert.NotContains(t, manifest, "super-secret-value")
	job := parseJobManifest(t, manifest)
	container := job.Spec.Template.Spec.Containers[0]
	assert.Equal(t, shareddomain.CurlImage, container.Image)

	env := map[string]jobEnv{}
	for _, e := range container.Env {
		env[e.Name] = e
	}
	assert.Equal(t, "http://nullus-seaweedfs-all-in-one.team-a.svc.cluster.local:8333", env["S3_ENDPOINT"].Value)
	require.NotNil(t, env["S3_SECRET_KEY"].ValueFrom)
	assert.Equal(t, domain.ProvisionedSeaweedFSSecret, env["S3_SECRET_KEY"].ValueFrom.SecretKeyRef.Name)
	assert.Equal(t, domain.SeaweedFSSecretKeyKey, env["S3_SECRET_KEY"].ValueFrom.SecretKeyRef.Key)
	require.NotNil(t, env["S3_ACCESS_KEY"].ValueFrom)
	assert.Equal(t, domain.SeaweedFSAccessKeyKey, env["S3_ACCESS_KEY"].ValueFrom.SecretKeyRef.Key)
	assert.Contains(t, strings.Join(container.Args, "\n"), "gitlab-registry")
}

// 외부 스토리지를 직접 입력한 경우는 클러스터에 그 값을 담은 Secret 이 없다.
func TestBucketBootstrapJobManifest_ExternalTargetUsesPlainEnvAndAddsScheme(t *testing.T) {
	target := objectStorageTarget{Endpoint: "s3.example.com:9000", AccessKey: "ak", SecretKey: "sk"}
	job := parseJobManifest(t, bucketBootstrapJobManifest("nullus", target, []string{"a"}))

	env := map[string]jobEnv{}
	for _, e := range job.Spec.Template.Spec.Containers[0].Env {
		env[e.Name] = e
	}
	assert.Equal(t, "http://s3.example.com:9000", env["S3_ENDPOINT"].Value)
	assert.Equal(t, "ak", env["S3_ACCESS_KEY"].Value)
	assert.Equal(t, "sk", env["S3_SECRET_KEY"].Value)
}

func TestS3EndpointURL(t *testing.T) {
	assert.Equal(t, "http://minio:9000", s3EndpointURL("minio:9000"))
	assert.Equal(t, "https://s3.example.com", s3EndpointURL("https://s3.example.com/"))
}

func TestEscapeJSONPathKey(t *testing.T) {
	out := escapeJSONPathKey("secret-key.name")
	assert.Equal(t, `secret\-key\.name`, out)
	assert.False(t, strings.Contains(out, " "))
}

type jobEnv struct {
	Name      string `yaml:"name"`
	Value     string `yaml:"value"`
	ValueFrom *struct {
		SecretKeyRef struct {
			Name string `yaml:"name"`
			Key  string `yaml:"key"`
		} `yaml:"secretKeyRef"`
	} `yaml:"valueFrom"`
}

type jobManifest struct {
	Kind string `yaml:"kind"`
	Spec struct {
		Template struct {
			Spec struct {
				Containers []struct {
					Image string   `yaml:"image"`
					Env   []jobEnv `yaml:"env"`
					Args  []string `yaml:"args"`
				} `yaml:"containers"`
			} `yaml:"spec"`
		} `yaml:"template"`
	} `yaml:"spec"`
}

// 매니페스트는 문자열로 조립한다. 들여쓰기 한 칸이 틀리면 apply 가 거절하므로
// YAML 로 다시 읽어 모양을 확인한다.
func parseJobManifest(t *testing.T, manifest string) jobManifest {
	t.Helper()
	var job jobManifest
	require.NoError(t, yaml.Unmarshal([]byte(manifest), &job), manifest)
	require.Equal(t, "Job", job.Kind)
	require.Len(t, job.Spec.Template.Spec.Containers, 1, manifest)
	return job
}
