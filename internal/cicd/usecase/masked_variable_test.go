package usecase

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// GitLab 은 마스킹 요건을 못 채우는 값을 masked 로 등록하면 400 으로 거부한다.
// 아래 기대값은 GitLab 17.7 API 에 직접 질의해 확인한 결과다.
func TestCanMaskVariableValue(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  bool
	}{
		{"충분히 길고 허용 문자만", "s3cretPassw0rd", true},
		{"Base64 확장 문자 허용", "abcd1234+/=@:.~", true},
		{"하이픈·언더스코어 허용", "robot-ci_acct", true},
		{"8자 미만은 불가 — Harbor 기본 계정명 admin", "admin", false},
		{"경계값 8자는 가능", "abcdefgh", true},
		{"경계값 7자는 불가", "abcdefg", false},
		{"공백 포함 불가", "with space12", false},
		{"달러 기호 불가 — 변수 치환으로 읽힌다", "robot$ci12345", false},
		{"줄바꿈 불가", "line1\nline2", false},
		{"빈 값 불가", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, canMaskVariableValue(tc.value))
		})
	}
}

// Harbor 기본 계정명 admin(5자)은 마스킹할 수 없다. 그럼에도 masked 로 밀면
// 등록이 400 으로 실패하고 CI build 가 docker login 에서 죽는다.
// 마스킹을 포기하더라도 변수는 반드시 등록되어야 한다.
func TestProvisionAppProject_RegistersShortUsernameUnmasked(t *testing.T) {
	scm, pipe, res := newFakeSCM(), newFakePipelineConfig(), harborResolver()
	uc := NewProvisionAppProject(scm, pipe, res)

	in := appInput()
	in.RegistryCredentials = map[string]string{
		"HARBOR_USERNAME": "admin",
		"HARBOR_PASSWORD": "s3cretPassw0rd",
	}

	out, err := uc.Execute(context.Background(), in)
	require.NoError(t, err)

	assert.Empty(t, out.MissingVariables, "마스킹 불가가 등록 실패로 이어지면 안 된다")

	user, ok := pipe.vars["HARBOR_USERNAME"]
	require.True(t, ok, "사용자명이 등록되지 않으면 docker login 이 실패한다")
	assert.Equal(t, "admin", user.Value)
	assert.False(t, user.Masked, "8자 미만이라 GitLab 이 masked 를 거부한다")

	pass, ok := pipe.vars["HARBOR_PASSWORD"]
	require.True(t, ok)
	assert.True(t, pass.Masked, "마스킹 가능한 비밀번호는 계속 가려야 한다")
}

// 마스킹을 끈 변수는 job 로그에 노출되므로 사용자에게 알려야 한다.
func TestProvisionAppProject_WarnsWhenCredentialCannotBeMasked(t *testing.T) {
	scm, pipe, res := newFakeSCM(), newFakePipelineConfig(), harborResolver()
	uc := NewProvisionAppProject(scm, pipe, res)

	in := appInput()
	in.RegistryCredentials = map[string]string{
		"HARBOR_USERNAME": "admin",
		"HARBOR_PASSWORD": "s3cretPassw0rd",
	}

	out, err := uc.Execute(context.Background(), in)
	require.NoError(t, err)

	require.NotEmpty(t, out.Warnings)
	assert.Contains(t, out.Warnings[0], "HARBOR_USERNAME")
	assert.False(t, pipe.vars["HARBOR_USERNAME"].Masked)
}
