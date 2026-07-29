package helm

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// secretPlaneSteps 는 provisioned Secret 을 만들어 내는 단계들이다.
var secretPlaneSteps = []string{
	"installing_openbao",
	"installing_external_secrets",
	"provisioning_secrets",
}

// 시크릿 평면은 authentication.provider 와 무관하게 항상 실행되어야 한다.
//
// PostgreSQL/MinIO 차트는 비밀번호를 values 로 받지 않고
// nullus-postgresql-credentials / nullus-minio-credentials 를
// existingSecret 으로 참조한다. 이 Secret 을 만드는 경로는
// provisioning_secrets 하나뿐이므로, 이 단계가 꺼지면 파드가
// FailedMount 로 기동하지 못하고 설치가 멈춘다.
func TestSecretPlane_EnabledRegardlessOfAuthProvider(t *testing.T) {
	cases := []struct {
		name string
		cfg  *domain.StackConfig
	}{
		{
			name: "authentication 설정 없음 (프런트엔드 기본값)",
			cfg:  &domain.StackConfig{},
		},
		{
			name: "provider 빈 문자열 (OpenBao 미선택)",
			cfg:  &domain.StackConfig{Authentication: &domain.AuthenticationConfig{Provider: ""}},
		},
		{
			name: "provider=openbao",
			cfg:  &domain.StackConfig{Authentication: &domain.AuthenticationConfig{Provider: "openbao"}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := NewOrchestrator(nil, nil, "nullus")
			o.SetStackConfig(*tc.cfg)

			for _, step := range secretPlaneSteps {
				assert.Truef(t, o.IsStepEnabled(step),
					"%s must stay enabled — it is the only producer of the provisioned secrets", step)
			}
		})
	}
}

// stackConfig 가 아직 설정되지 않은 상태에서도 시크릿 평면은 켜져 있어야 한다.
func TestSecretPlane_EnabledWhenStackConfigMissing(t *testing.T) {
	o := NewOrchestrator(nil, nil, "nullus")

	for _, step := range secretPlaneSteps {
		assert.Truef(t, o.IsStepEnabled(step), "%s must stay enabled without stack config", step)
	}
}

// provisioning_secrets 는 차트가 없는 단계다. chartSpecForStep 뒤에서 처리하면
// spec 조회 실패로 unknown step 이 되어 설치가 멈춘다.
func TestExecuteStep_ProvisioningSecrets_NeedsNoChartSpec(t *testing.T) {
	installer := &mockInstaller{}
	o := NewOrchestrator(installer, []byte("kubeconfig"), "nullus")

	_, hasChart := o.chartSpecForStep("provisioning_secrets")
	assert.False(t, hasChart, "provisioning_secrets is a chartless step")

	o.ResumeFromStep("stk_provisioning", "provisioning_secrets")
	err := o.ExecuteStep(context.Background(), "stk_provisioning", "provisioning_secrets", "A")

	require.NoError(t, err)
	assert.NotContains(t, installer.installed, "provisioning_secrets")
}
