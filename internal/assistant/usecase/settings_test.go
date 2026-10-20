package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/assistant/domain"
	"github.com/cloud-nullus/draft/internal/assistant/port"
)

func strptr(s string) *string { return &s }

func newSettingsFixture(initial ...domain.Settings) (*SettingsService, *fakeSettings, *fakeModels) {
	repo := &fakeSettings{byOrg: map[string]domain.Settings{}}
	for _, s := range initial {
		repo.byOrg[s.OrgID] = s
	}
	models := &fakeModels{llm: &scriptedLLM{replies: []domain.Message{{Role: domain.RoleAssistant, Content: "OK"}}}}
	return NewSettingsService(repo, models), repo, models
}

// 화면에는 키 원문이 가지 않는다 — 있는지와 끝 네 자리만.
func TestSettings_GetMasksTheKey(t *testing.T) {
	svc, _, _ := newSettingsFixture(readySettings)

	v, err := svc.Get(context.Background(), testOrg)
	require.NoError(t, err)
	assert.True(t, v.Enabled)
	assert.True(t, v.HasKey)
	assert.Equal(t, "••••1234", v.KeyHint)
	assert.Equal(t, "solar-pro4", v.EffectiveModel)
	assert.Len(t, v.Providers, len(domain.Providers()))

	empty, err := svc.Get(context.Background(), "org-none")
	require.NoError(t, err)
	assert.False(t, empty.Enabled)
	assert.Equal(t, domain.ProviderUpstage, empty.Provider, "처음엔 Upstage 를 고른 채로 보인다")
}

// 키를 다시 보내지 않으면 저장된 키를 그대로 쓴다. 공급자를 바꾸면 이전 공급자의 키는 버린다.
func TestSettings_UpdateKeepsOrReplacesTheKey(t *testing.T) {
	svc, repo, _ := newSettingsFixture(readySettings)
	ctx := context.Background()

	_, err := svc.Update(ctx, testOrg, "admin", UpdateSettings{Enabled: true, Provider: domain.ProviderUpstage, Model: "solar-mini"})
	require.NoError(t, err)
	assert.Equal(t, "up_abcd1234", repo.byOrg[testOrg].APIKey)
	assert.Equal(t, "solar-mini", repo.byOrg[testOrg].Model)
	assert.Equal(t, "admin", repo.byOrg[testOrg].UpdatedBy)

	_, err = svc.Update(ctx, testOrg, "admin", UpdateSettings{Enabled: true, Provider: domain.ProviderAnthropic})
	assert.ErrorIs(t, err, domain.ErrInvalidSettings, "공급자를 바꾸면 새 키가 필요하다")

	_, err = svc.Update(ctx, testOrg, "admin", UpdateSettings{Enabled: true, Provider: domain.ProviderAnthropic, APIKey: strptr(" sk-ant-new ")})
	require.NoError(t, err)
	assert.Equal(t, "sk-ant-new", repo.byOrg[testOrg].APIKey)

	// 빈 키를 보내면 지운다 — 꺼 둔 채로는 저장할 수 있다.
	_, err = svc.Update(ctx, testOrg, "admin", UpdateSettings{Enabled: false, Provider: domain.ProviderAnthropic, APIKey: strptr("")})
	require.NoError(t, err)
	assert.Empty(t, repo.byOrg[testOrg].APIKey)
}

// 저장하기 전에 실제로 한 번 물어 확인한다. 저장된 키를 써도 되고, 아무것도 저장하지 않는다.
func TestSettings_TestConnection(t *testing.T) {
	svc, repo, models := newSettingsFixture(readySettings)

	res, err := svc.Test(context.Background(), testOrg, UpdateSettings{Provider: domain.ProviderUpstage, Model: "solar-mini"})
	require.NoError(t, err)
	assert.True(t, res.OK)
	assert.Equal(t, "solar-mini", res.Model)
	require.Len(t, models.got, 1)
	assert.Equal(t, "up_abcd1234", models.got[0].APIKey, "저장된 키로 시험한다")
	assert.Empty(t, repo.saved)
}

func TestSettings_TestConnectionReportsFailureWithoutTheKey(t *testing.T) {
	svc, _, models := newSettingsFixture()
	models.llm = &scriptedLLM{err: errors.New("모델 응답 401: invalid key sk-leak-1234567")}

	res, err := svc.Test(context.Background(), testOrg, UpdateSettings{Provider: domain.ProviderOpenAI, Model: "m", APIKey: strptr("sk-leak-1234567")})
	require.NoError(t, err)
	assert.False(t, res.OK)
	assert.Contains(t, res.Error, "401")
	assert.NotContains(t, res.Error, "sk-leak-1234567")
}

func TestSettings_TestRejectsInvalidSettings(t *testing.T) {
	svc, _, _ := newSettingsFixture()
	_, err := svc.Test(context.Background(), testOrg, UpdateSettings{Provider: domain.ProviderUpstage})
	assert.ErrorIs(t, err, domain.ErrInvalidSettings, "키가 없다")
}

var _ port.SettingsRepository = (*fakeSettings)(nil)

// 주소를 바꾸면 저장된 키를 새 주소로 보내지 않는다 — 공급자와 주소가 둘 다 같을 때만 키를 이어 쓴다.
// 그렇지 않으면 저장된 키를 아무 서버로나 보내 빼낼 수 있다(연결 테스트로도).
func TestSettings_ChangingTheAddressRequiresTheKeyAgain(t *testing.T) {
	svc, repo, models := newSettingsFixture(readySettings)
	ctx := context.Background()

	_, err := svc.Test(ctx, testOrg, UpdateSettings{Provider: domain.ProviderUpstage, BaseURL: "https://evil.example/v1"})
	assert.ErrorIs(t, err, domain.ErrInvalidSettings, "키 없이 다른 주소로 시험할 수 없다")
	assert.Empty(t, models.got, "저장된 키로 모델을 만들지 않는다")

	_, err = svc.Update(ctx, testOrg, "admin", UpdateSettings{Enabled: true, Provider: domain.ProviderUpstage, BaseURL: "https://evil.example/v1"})
	assert.ErrorIs(t, err, domain.ErrInvalidSettings)
	assert.Equal(t, "", repo.byOrg[testOrg].BaseURL, "저장도 하지 않는다")

	// 기본 주소를 그대로 적어 넣은 것은 같은 주소다.
	_, err = svc.Update(ctx, testOrg, "admin", UpdateSettings{Enabled: true, Provider: domain.ProviderUpstage, BaseURL: "https://api.upstage.ai/v1/"})
	require.NoError(t, err)
	assert.Equal(t, "up_abcd1234", repo.byOrg[testOrg].APIKey)
}

// 저장된 키를 읽지 못하면(ENCRYPTION_KEY 가 바뀜) 설정은 보이고, 키를 다시 넣으라고 알린다.
func TestSettings_UnreadableKeyIsReportedAndCanBeReplaced(t *testing.T) {
	broken := readySettings
	broken.APIKey, broken.KeyUnreadable = "", true
	svc, repo, _ := newSettingsFixture(broken)

	v, err := svc.Get(context.Background(), testOrg)
	require.NoError(t, err)
	assert.True(t, v.KeyUnreadable)
	assert.False(t, v.HasKey)

	_, err = svc.Update(context.Background(), testOrg, "admin", UpdateSettings{Enabled: true, Provider: domain.ProviderUpstage, APIKey: strptr("up_new_key_5678")})
	require.NoError(t, err)
	assert.Equal(t, "up_new_key_5678", repo.byOrg[testOrg].APIKey)
	assert.False(t, repo.byOrg[testOrg].KeyUnreadable)
}
