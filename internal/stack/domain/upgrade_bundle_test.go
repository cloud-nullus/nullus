package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var bundleNow = time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC)

func validBundleInput() NewUpgradeBundleInput {
	return NewUpgradeBundleInput{
		Tool: "argocd", ReleaseName: "argo-cd", ChartName: "argo-cd", RepoURL: "https://argoproj.github.io/argo-helm",
		SourceChartVersion: "7.7.16", SourceAppVersion: "v2.13.3", TargetChartVersion: "7.7.22", TargetAppVersion: "v2.13.4",
		RiskLevel: "low", ExpectedDisruption: "API 서버 · 컨트롤러가 차례로 재시작(약 1~2분)",
		ReleaseNotes: "https://github.com/argoproj/argo-cd/releases/tag/v2.13.4", RequiresBackup: true,
		SupportedArchitectures: []string{"amd64", "arm64"}, FixesCVEs: []string{" cve-2025-12345 ", "CVE-2025-12345", "CVE-2024-1234"},
		Status: UpgradeBundleVerified, RehearsalConfirmed: true, Now: bundleNow,
	}
}

func TestNewUpgradeBundle_DerivesIDAndNormalizes(t *testing.T) {
	b, err := NewUpgradeBundle(validBundleInput())
	require.NoError(t, err)
	assert.Equal(t, "argocd-7.7.16-7.7.22", b.ID, "ID 를 비우면 도구-출발-대상으로 만든다")
	assert.Equal(t, UpgradeBundleVerified, b.Status)
	assert.Equal(t, []string{"CVE-2024-1234", "CVE-2025-12345"}, b.FixesCVEs, "CVE 는 대문자로 맞추고 중복을 빼고 정렬한다")
	assert.Equal(t, []string{"amd64", "arm64"}, b.SupportedArchitectures)
	assert.Equal(t, bundleNow, b.CreatedAt)
	assert.True(t, b.RequiresBackup)
}

func TestNewUpgradeBundle_DefaultsToDraft(t *testing.T) {
	in := validBundleInput()
	in.Status, in.RehearsalConfirmed = "", false
	b, err := NewUpgradeBundle(in)
	require.NoError(t, err)
	assert.Equal(t, UpgradeBundleDraft, b.Status, "상태를 비우면 draft — 리허설 없이 실행 가능한 경로가 생기지 않는다")
}

func TestNewUpgradeBundle_VerifiedRequiresRehearsalConfirmation(t *testing.T) {
	in := validBundleInput()
	in.RehearsalConfirmed = false
	_, err := NewUpgradeBundle(in)
	assert.ErrorIs(t, err, ErrUpgradeBundleRehearsalRequired)
}

func TestNewUpgradeBundle_RejectsInvalidInput(t *testing.T) {
	cases := map[string]func(in *NewUpgradeBundleInput){
		"지원하지 않는 도구":  func(in *NewUpgradeBundleInput) { in.Tool = "gitlab" },
		"릴리스 이름 없음":   func(in *NewUpgradeBundleInput) { in.ReleaseName = " " },
		"차트 이름 없음":    func(in *NewUpgradeBundleInput) { in.ChartName = "" },
		"저장소 주소 없음":   func(in *NewUpgradeBundleInput) { in.RepoURL = "" },
		"저장소 주소 스킴":   func(in *NewUpgradeBundleInput) { in.RepoURL = "ftp://charts.example.com" },
		"저장소 주소 형식":   func(in *NewUpgradeBundleInput) { in.RepoURL = "https://" },
		"출발 버전 형식":    func(in *NewUpgradeBundleInput) { in.SourceChartVersion = "latest" },
		"대상 버전 형식":    func(in *NewUpgradeBundleInput) { in.TargetChartVersion = "7.7.x" },
		"대상이 출발보다 낮음": func(in *NewUpgradeBundleInput) { in.TargetChartVersion = "7.7.10" },
		"대상이 출발과 같음":  func(in *NewUpgradeBundleInput) { in.TargetChartVersion = "7.7.16" },
		"위험도":         func(in *NewUpgradeBundleInput) { in.RiskLevel = "critical" },
		"아키텍처":        func(in *NewUpgradeBundleInput) { in.SupportedArchitectures = []string{"amd64", "riscv"} },
		"CVE 형식":      func(in *NewUpgradeBundleInput) { in.FixesCVEs = []string{"GHSA-xxxx"} },
		"ID 형식":       func(in *NewUpgradeBundleInput) { in.ID = "Argo CD 7.7.22" },
		"등록 때 폐기 상태":  func(in *NewUpgradeBundleInput) { in.Status = UpgradeBundleDeprecated },
		"등록 때 회수 상태":  func(in *NewUpgradeBundleInput) { in.Status = UpgradeBundleRevoked },
		"릴리스 노트 주소 형식": func(in *NewUpgradeBundleInput) {
			in.ReleaseNotes = "http://exa mple.com/notes"
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			in := validBundleInput()
			mutate(&in)
			_, err := NewUpgradeBundle(in)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrUpgradeBundleInvalid)
		})
	}
}

func TestNewUpgradeBundle_AcceptsOCIRepositoryAndExplicitID(t *testing.T) {
	in := validBundleInput()
	in.ID, in.RepoURL = "argocd-patch-2026-10", "oci://registry.internal/charts"
	b, err := NewUpgradeBundle(in)
	require.NoError(t, err)
	assert.Equal(t, "argocd-patch-2026-10", b.ID)
	assert.Equal(t, "oci://registry.internal/charts", b.RepoURL)
}

func TestUpgradeBundle_ChangeStatus(t *testing.T) {
	type tc struct {
		from, to  string
		confirmed bool
		want      error
	}
	cases := map[string]tc{
		"draft → verified(확인)":      {UpgradeBundleDraft, UpgradeBundleVerified, true, nil},
		"draft → verified(확인 없음)":   {UpgradeBundleDraft, UpgradeBundleVerified, false, ErrUpgradeBundleRehearsalRequired},
		"draft → revoked":           {UpgradeBundleDraft, UpgradeBundleRevoked, false, nil},
		"draft → deprecated":        {UpgradeBundleDraft, UpgradeBundleDeprecated, false, ErrUpgradeBundleStatusTransition},
		"verified → deprecated":     {UpgradeBundleVerified, UpgradeBundleDeprecated, false, nil},
		"verified → revoked":        {UpgradeBundleVerified, UpgradeBundleRevoked, false, nil},
		"verified → verified":       {UpgradeBundleVerified, UpgradeBundleVerified, true, ErrUpgradeBundleStatusTransition},
		"deprecated → verified(확인)": {UpgradeBundleDeprecated, UpgradeBundleVerified, true, nil},
		"deprecated → revoked":      {UpgradeBundleDeprecated, UpgradeBundleRevoked, false, nil},
		"revoked → verified":        {UpgradeBundleRevoked, UpgradeBundleVerified, true, ErrUpgradeBundleStatusTransition},
		"revoked → deprecated":      {UpgradeBundleRevoked, UpgradeBundleDeprecated, false, ErrUpgradeBundleStatusTransition},
		"어디서든 → draft":              {UpgradeBundleVerified, UpgradeBundleDraft, false, ErrUpgradeBundleStatusTransition},
		"모르는 상태":                    {UpgradeBundleVerified, "archived", false, ErrUpgradeBundleStatusTransition},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			b := &UpgradeBundle{ID: "b", Status: c.from}
			err := b.ChangeStatus(c.to, c.confirmed)
			if c.want == nil {
				require.NoError(t, err)
				assert.Equal(t, c.to, b.Status)
				return
			}
			require.True(t, errors.Is(err, c.want), "got %v", err)
			assert.Equal(t, c.from, b.Status, "실패하면 상태가 그대로다")
		})
	}
}
