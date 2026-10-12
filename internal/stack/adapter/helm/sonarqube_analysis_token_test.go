package helm

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/shared/secrets"
	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// CI 가 분석에 쓸 토큰은 스택 설치가 발급해 OpenBao 에 둔다. 파이프라인을 만드는 쪽(cicd)은
// 그 값을 읽어 시크릿 CI 변수로 건다 — SonarQube 에 직접 닿지 않아도 된다.
//
// 발급은 클러스터 안 Job 이 한다. 플랫폼 API 는 클러스터 밖에서 돌 수 있어 SonarQube 에
// 닿는다는 보장이 없다.

func TestSonarQubeTokenJob_IssuesGlobalAnalysisToken(t *testing.T) {
	manifest := sonarqubeTokenJobManifest("nullus-demo", "demo", false)

	assert.Contains(t, manifest, "http://sonarqube.nullus-demo.svc.cluster.local:9000")
	assert.Contains(t, manifest, "/api/user_tokens/search")
	assert.Contains(t, manifest, "/api/user_tokens/generate")
	assert.Contains(t, manifest, "type=GLOBAL_ANALYSIS_TOKEN",
		"사용자 토큰을 주면 CI 가 관리자 권한을 갖는다 — 분석 권한만 준다")
	assert.Contains(t, manifest, "NAME="+sonarqubeAnalysisTokenName)
	// 관리자 비밀번호는 Secret 에서 온다. 매니페스트에 평문이 남지 않는다.
	assert.Contains(t, manifest, "name: "+domain.SonarQubeSecret)
	assert.Contains(t, manifest, "key: "+domain.SonarQubeAdminPasswordKey)
	assert.NotContains(t, manifest, "/api/user_tokens/revoke",
		"이미 있는 토큰을 지우면 그 토큰을 쓰는 파이프라인이 모두 인증에서 죽는다")
}

// 토큰을 표준 출력에 싣지 않는다. 스택에 로그 수집(OTel agent → Loki)이 있으면 파드 로그가
// 그대로 보관된다 — Job 을 지워도 사본이 남는다. 파드 안 파일에 두고 오케스트레이터가 exec 로
// 읽은 뒤 Job 을 지운다.
func TestSonarQubeTokenJob_KeepsTokenOutOfLogs(t *testing.T) {
	manifest := sonarqubeTokenJobManifest("nullus-demo", "demo", false)

	assert.Contains(t, manifest, sonarTokenResultFile)
	for _, line := range strings.Split(manifest, "\n") {
		if strings.Contains(line, "echo") && strings.Contains(line, "$TOKEN") {
			assert.Contains(t, line, "> "+sonarTokenResultFile, "토큰이 담긴 echo 는 파일로만 간다: %s", line)
		}
	}
	// Job 을 지우면 파드가 바로 끝난다. PID 1 인 sh 는 처리기 없는 신호를 무시해, 그대로면
	// 유예 시간 내내 남아 다음 실행이 그 파드의 결과를 읽는다.
	assert.Contains(t, manifest, "trap 'exit 0' TERM INT")
	assert.NotContains(t, manifest, "; sleep 600;", "포그라운드 sleep 은 신호를 받아도 깨지 않는다")
	// 오케스트레이터가 읽기 전에 죽어도 Job 이 영원히 남지 않는다.
	assert.Contains(t, manifest, "ttlSecondsAfterFinished:")
	assert.Contains(t, manifest, "activeDeadlineSeconds:")
}

// 값을 잃었을 때만 다시 발급한다. 이전 토큰은 폐기하지 않고 새 이름으로 하나 더 만든다 —
// 이전 토큰은 기존 파이프라인 변수에 사본이 남아 있을 수 있어, 폐기하면 그 파이프라인이 모두
// 인증에서 죽는다.
func TestSonarQubeTokenJob_RotateIssuesNewNameWithoutRevoking(t *testing.T) {
	manifest := sonarqubeTokenJobManifest("nullus-demo", "demo", true)
	assert.NotContains(t, manifest, "/api/user_tokens/revoke")
	assert.Contains(t, manifest, "/api/user_tokens/generate")
	assert.Contains(t, manifest, `NAME="$NAME-$(date +%Y%m%d%H%M%S)"`)
}

func TestParseSonarQubeTokenResult(t *testing.T) {
	got, err := parseSonarQubeTokenResult("TOKEN=sqa_abc123\n")
	require.NoError(t, err)
	assert.Equal(t, sonarTokenJobResult{Token: "sqa_abc123"}, got)

	got, err = parseSonarQubeTokenResult("EXISTS\n")
	require.NoError(t, err)
	assert.Equal(t, sonarTokenJobResult{Exists: true}, got)

	_, err = parseSonarQubeTokenResult("ERROR=token response did not carry a token\n")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "token response did not carry a token")

	_, err = parseSonarQubeTokenResult("")
	assert.Error(t, err, "결과가 없으면 토큰 없이 넘어가지 않는다")
}

type memSecretWriter struct {
	values map[string]string
	getErr error
	puts   int
}

func (m *memSecretWriter) PutToken(_ context.Context, path, value string) error {
	m.values[path] = value
	m.puts++
	return nil
}

func (m *memSecretWriter) GetToken(_ context.Context, path string) (string, error) {
	if m.getErr != nil {
		return "", m.getErr
	}
	v, ok := m.values[path]
	if !ok {
		return "", &secrets.StatusError{Status: http.StatusNotFound}
	}
	return v, nil
}

const testTokenPath = "kv/nullus/dev/org/security/sonarqube/analysis-token"

func recordRuns(results map[bool]sonarTokenJobResult, rotates *[]bool) func(context.Context, bool) (sonarTokenJobResult, error) {
	return func(_ context.Context, rotate bool) (sonarTokenJobResult, error) {
		*rotates = append(*rotates, rotate)
		return results[rotate], nil
	}
}

func TestSyncSonarQubeAnalysisToken_StoresNewToken(t *testing.T) {
	store := &memSecretWriter{values: map[string]string{}}
	var rotates []bool
	err := syncSonarQubeAnalysisToken(context.Background(), store, testTokenPath,
		recordRuns(map[bool]sonarTokenJobResult{false: {Token: "sqa_new"}}, &rotates))
	require.NoError(t, err)
	assert.Equal(t, "sqa_new", store.values[testTokenPath])
	assert.Equal(t, []bool{false}, rotates)
}

// 다시 돌려도 같은 결과다. 이미 발급했고 OpenBao 에 값이 있으면 그대로 둔다.
func TestSyncSonarQubeAnalysisToken_KeepsExisting(t *testing.T) {
	store := &memSecretWriter{values: map[string]string{testTokenPath: "sqa_current"}}
	var rotates []bool
	err := syncSonarQubeAnalysisToken(context.Background(), store, testTokenPath,
		recordRuns(map[bool]sonarTokenJobResult{false: {Exists: true}}, &rotates))
	require.NoError(t, err)
	assert.Equal(t, "sqa_current", store.values[testTokenPath])
	assert.Zero(t, store.puts)
	assert.Equal(t, []bool{false}, rotates)
}

// SonarQube 에는 토큰이 있는데 OpenBao 에 값이 없다(금고 재초기화 등). 값을 되찾을 길이 없으니
// 새 이름으로 하나 더 발급한다.
func TestSyncSonarQubeAnalysisToken_ReissuesWhenValueLost(t *testing.T) {
	for name, store := range map[string]*memSecretWriter{
		"경로가 없다": {values: map[string]string{}},
		"값이 비었다": {values: map[string]string{testTokenPath: " "}},
	} {
		t.Run(name, func(t *testing.T) {
			var rotates []bool
			err := syncSonarQubeAnalysisToken(context.Background(), store, testTokenPath,
				recordRuns(map[bool]sonarTokenJobResult{false: {Exists: true}, true: {Token: "sqa_reissued"}}, &rotates))
			require.NoError(t, err)
			assert.Equal(t, []bool{false, true}, rotates)
			assert.Equal(t, "sqa_reissued", store.values[testTokenPath])
		})
	}
}

// OpenBao 를 읽지 못한 것(봉인·장애·권한)은 값을 잃은 것과 다르다. 그때 다시 발급하면 기록도
// 실패해 새 토큰을 잃고, 다음 실행은 옛 값을 그대로 믿는다. 멈추고 다음 시도에 맡긴다.
func TestSyncSonarQubeAnalysisToken_StopsWhenOpenBaoUnreadable(t *testing.T) {
	store := &memSecretWriter{values: map[string]string{}, getErr: errors.New("openbao request failed: status=503 body=sealed")}
	var rotates []bool
	err := syncSonarQubeAnalysisToken(context.Background(), store, testTokenPath,
		recordRuns(map[bool]sonarTokenJobResult{false: {Exists: true}}, &rotates))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sealed")
	assert.Equal(t, []bool{false}, rotates, "읽지 못했다고 다시 발급하지 않는다")
	assert.Zero(t, store.puts)
}

func TestSyncSonarQubeAnalysisToken_JobFailureStops(t *testing.T) {
	store := &memSecretWriter{values: map[string]string{}}
	err := syncSonarQubeAnalysisToken(context.Background(), store, testTokenPath, func(context.Context, bool) (sonarTokenJobResult, error) {
		return sonarTokenJobResult{}, errors.New("job timeout")
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "job timeout")
	assert.Zero(t, store.puts)
}
