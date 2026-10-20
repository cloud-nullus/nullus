package assistant

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/assistant/domain"
	"github.com/cloud-nullus/draft/internal/assistant/usecase"
)

type memSettings struct{ byOrg map[string]domain.Settings }

func (m *memSettings) Get(_ context.Context, orgID string) (*domain.Settings, error) {
	s, ok := m.byOrg[orgID]
	if !ok {
		return nil, nil
	}
	return &s, nil
}
func (m *memSettings) Save(_ context.Context, s domain.Settings) error {
	m.byOrg[s.OrgID] = s
	return nil
}
func (m *memSettings) Delete(_ context.Context, orgID string) error {
	delete(m.byOrg, orgID)
	return nil
}

// 관리 화면에서 등록하기 전에는 꺼져 있고, 등록하면 그 조직만 켜진다.
func TestModule_TurnsOnWhenTheOrgRegistersAKey(t *testing.T) {
	m := New(Deps{Settings: &memSettings{byOrg: map[string]domain.Settings{}}, ServerURL: "http://127.0.0.1:8090", Version: "dev"})
	ctx := context.Background()

	st, err := m.Chat.Status(ctx, "org-1")
	require.NoError(t, err)
	assert.False(t, st.Enabled)

	key := "up_abcd1234"
	_, err = m.Settings.Update(ctx, "org-1", "admin", usecase.UpdateSettings{Enabled: true, Provider: domain.ProviderUpstage, APIKey: &key})
	require.NoError(t, err)

	st, err = m.Chat.Status(ctx, "org-1")
	require.NoError(t, err)
	assert.Equal(t, usecase.Status{Enabled: true, Provider: "upstage", Model: "solar-pro4"}, st)
	other, err := m.Chat.Status(ctx, "org-2")
	require.NoError(t, err)
	assert.False(t, other.Enabled)
}
