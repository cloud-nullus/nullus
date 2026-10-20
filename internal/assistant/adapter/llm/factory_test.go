package llm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/assistant/domain"
)

// 공급자의 프로토콜에 맞는 클라이언트를 고르고, 비어 있는 주소 · 모델은 공급자 기본값을 쓴다.
func TestFactory_PicksClientByProtocol(t *testing.T) {
	f := Factory{}

	c, err := f.New(domain.Settings{Provider: domain.ProviderUpstage, APIKey: "k"})
	require.NoError(t, err)
	oc, ok := c.(*Client)
	require.True(t, ok)
	assert.Equal(t, "https://api.upstage.ai/v1", oc.cfg.BaseURL)
	assert.Equal(t, "solar-pro4", oc.cfg.Model)

	c, err = f.New(domain.Settings{Provider: domain.ProviderAnthropic, APIKey: "k"})
	require.NoError(t, err)
	ac, ok := c.(*AnthropicClient)
	require.True(t, ok)
	assert.Equal(t, "https://api.anthropic.com", ac.cfg.BaseURL)

	c, err = f.New(domain.Settings{Provider: domain.ProviderGemini, APIKey: "k", Model: "gemini-x"})
	require.NoError(t, err)
	assert.Equal(t, "https://generativelanguage.googleapis.com/v1beta/openai", c.(*Client).cfg.BaseURL)

	_, err = f.New(domain.Settings{Provider: "acme"})
	assert.Error(t, err)
}
