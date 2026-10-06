package helm

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

// 스택을 지울 때 공용 릴리스는 values 의 설치 표시로 Nullus 것인지 가린다. 표시가 빠지면 새로 설치한
// cert-manager 가 옛 설치의 지문에도 맞지 않을 수 있어(관리자 기본값이 구조를 바꾸면) 회수되지 않는다.
func TestValuesForStep_SharedReleasesCarryInstallMarker(t *testing.T) {
	for _, tc := range []struct {
		step    string
		release string
	}{
		{stepInstallingCertManager, domain.CertManagerReleaseName},
		{"installing_metrics_server", domain.MetricsServerReleaseName},
	} {
		t.Run(tc.step, func(t *testing.T) {
			o := NewOrchestrator(nil, nil, "nullus-demo")
			spec, ok := defaultChartSpecForStep(tc.step)
			require.True(t, ok)

			values := o.valuesForStep(tc.step, spec)

			// 옛 설치 지문으로도 통과하므로 표시 자리를 직접 본다.
			path, ok := domain.InstallMarkerValuePath(tc.release)
			require.True(t, ok)
			marker, found := lookupValue(values, path)
			assert.True(t, found, "설치 표시가 values 에 없다")
			assert.Equal(t, domain.InstalledByNullus, marker)
		})
	}
}

// 사용자 오버라이드가 표시를 지워도 플랫폼이 다시 단다. 지워진 채 설치되면 삭제 때 고객 것으로 보고 남긴다.
func TestEnforcePlatformOwnedValues_RestoresInstallMarker(t *testing.T) {
	o := NewOrchestrator(nil, nil, "nullus-demo")
	overridden := map[string]any{
		"global": map[string]any{"commonLabels": map[string]any{domain.LabelInstalledBy: "edited"}},
	}

	values := o.enforcePlatformOwnedValues(stepInstallingCertManager, overridden)

	path, _ := domain.InstallMarkerValuePath(domain.CertManagerReleaseName)
	marker, _ := lookupValue(values, path)
	assert.Equal(t, domain.InstalledByNullus, marker)
}

const installMarkerTestKubeconfig = `apiVersion: v1
kind: Config
clusters:
- name: kind
  cluster:
    server: https://127.0.0.1:6443
contexts:
- name: kind
  context: {cluster: kind, user: kind}
current-context: kind
users:
- name: kind
  user: {token: t}
`

// fakeSharedResourceKubectl 은 호출 인자를 남기고, create·apply 의 표준입력을 따로 남기는 kubectl 이다.
// get crd 에는 crdOut 을 돌려준다. create 는 createResult 대로 끝난다 — "exists" 면 AlreadyExists,
// "fail" 이면 다른 오류다.
func fakeSharedResourceKubectl(t *testing.T, createResult, crdOut string) (callsLog, stdinLog string) {
	t.Helper()
	dir := t.TempDir()
	callsLog = filepath.Join(dir, "calls.log")
	stdinLog = filepath.Join(dir, "stdin.log")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "crd.out"), []byte(crdOut), 0o644))
	create := "cat >> '" + stdinLog + "'; exit 0"
	switch createResult {
	case "exists":
		create = "cat > /dev/null; echo 'Error from server (AlreadyExists): error when creating \"STDIN\": " +
			"gatewayclasses.gateway.networking.k8s.io \"envoy\" already exists' >&2; exit 1"
	case "fail":
		create = "cat > /dev/null; echo 'Unable to connect to the server: dial tcp: i/o timeout' >&2; exit 1"
	}
	script := "#!/bin/sh\n" +
		"echo \"$*\" >> '" + callsLog + "'\n" +
		"case \" $* \" in\n" +
		"  *\" create \"*) " + create + " ;;\n" +
		"  *\" apply \"*) cat >> '" + stdinLog + "'; exit 0 ;;\n" +
		"  *\" get crd \"*) cat '" + filepath.Join(dir, "crd.out") + "'; exit 0 ;;\n" +
		"esac\nexit 0\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "kubectl"), []byte(script), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return callsLog, stdinLog
}

func readLog(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return ""
	}
	require.NoError(t, err)
	return string(raw)
}

func testContext(t *testing.T) context.Context {
	t.Helper()
	// 막 만든 스크립트의 첫 실행은 부하가 높은 macOS 에서 수 초 걸린다.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// 이미 있는 GatewayClass envoy 는 고객 것이거나 지난 스택의 것이다. 덮어쓰면 고객 것이 이 설치기의 것으로
// 바뀌고(표시까지 달려) 삭제 때 Nullus 것으로 보고 지운다. 그래서 apply 가 아니라 create 로 만든다 — 있는지
// 묻는 조회가 실패해도 남의 것을 덮어쓰지 않는다.
func TestEnsureEnvoyGatewayClass_LeavesExistingClassAlone(t *testing.T) {
	callsLog, _ := fakeSharedResourceKubectl(t, "exists", "")
	o := NewOrchestrator(nil, []byte(installMarkerTestKubeconfig), "nullus-demo")

	require.NoError(t, o.ensureEnvoyGatewayClass(testContext(t)))

	assert.NotContains(t, readLog(t, callsLog), "apply")
}

func TestEnsureEnvoyGatewayClass_CreatesWithInstallMarkerWhenMissing(t *testing.T) {
	callsLog, stdinLog := fakeSharedResourceKubectl(t, "ok", "")
	o := NewOrchestrator(nil, []byte(installMarkerTestKubeconfig), "nullus-demo")

	require.NoError(t, o.ensureEnvoyGatewayClass(testContext(t)))

	assert.Contains(t, readLog(t, callsLog), "create -f -")
	manifest := readLog(t, stdinLog)
	assert.Contains(t, manifest, "kind: GatewayClass")
	assert.Contains(t, manifest, domain.LabelInstalledBy+": "+domain.InstalledByNullus)
}

func TestEnsureEnvoyGatewayClass_ReportsOtherFailures(t *testing.T) {
	fakeSharedResourceKubectl(t, "fail", "")
	o := NewOrchestrator(nil, []byte(installMarkerTestKubeconfig), "nullus-demo")

	assert.Error(t, o.ensureEnvoyGatewayClass(testContext(t)))
}

// 게이트웨이 단계가 새로 깐 CRD 에만 표시한다. 원래 있던 CRD(고객 것)와 게이트웨이와 무관한 CRD 는
// 건드리지 않는다. Envoy 차트가 crds/ 로 까는 실험용·Envoy CRD 도 빠지면 안 된다.
func TestMarkGatewayCRDsCreatedSince_LabelsOnlyNewGatewayCRDs(t *testing.T) {
	after := strings.Join([]string{
		"customresourcedefinition.apiextensions.k8s.io/gateways.gateway.networking.k8s.io",
		"customresourcedefinition.apiextensions.k8s.io/httproutes.gateway.networking.k8s.io",
		"customresourcedefinition.apiextensions.k8s.io/xlistenersets.gateway.networking.x-k8s.io",
		"customresourcedefinition.apiextensions.k8s.io/backends.gateway.envoyproxy.io",
		"customresourcedefinition.apiextensions.k8s.io/certificates.cert-manager.io",
	}, "\n") + "\n"
	callsLog, _ := fakeSharedResourceKubectl(t, "", after)
	o := NewOrchestrator(nil, []byte(installMarkerTestKubeconfig), "nullus-demo")
	before := map[string]bool{"gateways.gateway.networking.k8s.io": true}

	o.markGatewayCRDsCreatedSince(testContext(t), before)

	calls := readLog(t, callsLog)
	assert.Contains(t, calls, "label crd backends.gateway.envoyproxy.io httproutes.gateway.networking.k8s.io "+
		"xlistenersets.gateway.networking.x-k8s.io "+domain.LabelInstalledBy+"="+domain.InstalledByNullus+" --overwrite")
	assert.NotContains(t, calls, "label crd gateways.gateway.networking.k8s.io")
	assert.NotContains(t, calls, "certificates.cert-manager.io "+domain.LabelInstalledBy)
}

// 설치 전 목록을 읽지 못했으면 표시하지 않는다 — 원래 있던 CRD 에 표시를 다는 것보다 새로 깐 CRD 를
// 남기는 편이 낫다.
func TestMarkGatewayCRDsCreatedSince_SkipsWithoutSnapshot(t *testing.T) {
	callsLog, _ := fakeSharedResourceKubectl(t, "", "customresourcedefinition.apiextensions.k8s.io/gateways.gateway.networking.k8s.io\n")
	o := NewOrchestrator(nil, []byte(installMarkerTestKubeconfig), "nullus-demo")

	o.markGatewayCRDsCreatedSince(testContext(t), nil)

	assert.NotContains(t, readLog(t, callsLog), "label")
}

func TestCRDNameSet_IgnoresNoticeLines(t *testing.T) {
	names := crdNameSet("customresourcedefinition.apiextensions.k8s.io/gateways.gateway.networking.k8s.io\n" +
		"No resources found\n\n")

	assert.Equal(t, map[string]bool{"gateways.gateway.networking.k8s.io": true}, names)
}

// valuesMockInstaller 는 이미 있는 릴리스의 values 를 돌려준다.
type valuesMockInstaller struct {
	*mockInstaller
	existing map[string]any
}

func (m *valuesMockInstaller) GetValues(context.Context, string, string) (map[string]any, error) {
	return m.existing, nil
}

func markerValue(t *testing.T, release string, values map[string]any) (any, bool) {
	t.Helper()
	path, ok := domain.InstallMarkerValuePath(release)
	require.True(t, ok)
	return lookupValue(values, path)
}

// 설치 표시는 Nullus 가 새로 까는 릴리스에만 단다. 재사용 판정이 놓친 고객의 cert-manager(cainjector 를 끈
// 설치 등)를 Install 이 업그레이드하면서 표시를 달면, 스택을 지울 때 고객 것을 회수한다.
func TestWithInstallMarkerIfOwned(t *testing.T) {
	nullusValues := func() map[string]any {
		return map[string]any{
			"installCRDs": true,
			"global":      map[string]any{"commonLabels": map[string]any{domain.LabelInstalledBy: domain.InstalledByNullus}},
		}
	}
	cases := []struct {
		name      string
		installer port.HelmInstaller
		want      bool
	}{
		{"새로 까는 릴리스", &mockInstaller{strictStatus: true}, true},
		{"이미 있는 Nullus 릴리스(재시도)", &valuesMockInstaller{mockInstaller: &mockInstaller{}, existing: nullusValues()}, true},
		{"이미 있는 고객 릴리스", &valuesMockInstaller{mockInstaller: &mockInstaller{}, existing: map[string]any{"installCRDs": true}}, false},
		{"values 를 읽을 수 없는 기존 릴리스", &mockInstaller{}, false},
		{"상태를 확인하지 못함", &mockInstaller{strictStatus: true, failStatus: errors.New("connection refused")}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := NewOrchestrator(tc.installer, nil, "nullus-demo")

			values := o.withInstallMarkerIfOwned(context.Background(), domain.CertManagerReleaseName, "cert-manager", nullusValues())

			marker, found := markerValue(t, domain.CertManagerReleaseName, values)
			assert.Equal(t, tc.want, found && marker == domain.InstalledByNullus)
			if !tc.want {
				assert.NotContains(t, values, "global", "빈 global 을 남기지 않는다")
				assert.Equal(t, true, values["installCRDs"], "표시 말고는 건드리지 않는다")
			}
		})
	}
}
