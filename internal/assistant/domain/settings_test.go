package domain

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProviders_CoverVendorsAndProtocols(t *testing.T) {
	ids := map[string]Protocol{}
	for _, p := range Providers() {
		ids[p.ID] = p.Protocol
	}
	assert.Equal(t, map[string]Protocol{
		ProviderUpstage: ProtocolOpenAI, ProviderOpenAI: ProtocolOpenAI, ProviderAnthropic: ProtocolAnthropic,
		ProviderGemini: ProtocolOpenAI, ProviderOpenAICompatible: ProtocolOpenAI,
	}, ids)
}

func TestSettings_EffectiveValuesFallBackToProviderDefaults(t *testing.T) {
	s := Settings{Provider: ProviderUpstage}
	assert.Equal(t, "https://api.upstage.ai/v1", s.EffectiveBaseURL())
	assert.Equal(t, "solar-pro4", s.EffectiveModel())

	s = Settings{Provider: ProviderUpstage, BaseURL: "https://proxy.example/v1", Model: "solar-mini"}
	assert.Equal(t, "https://proxy.example/v1", s.EffectiveBaseURL())
	assert.Equal(t, "solar-mini", s.EffectiveModel())
}

func TestValidateSettings(t *testing.T) {
	ok := []Settings{
		{Enabled: true, Provider: ProviderUpstage, APIKey: "up_x"},
		{Enabled: true, Provider: ProviderAnthropic, APIKey: "sk-ant-x"},
		{Enabled: true, Provider: ProviderOpenAI, Model: "gpt-x", APIKey: "sk-x"},
		// 사내 서버는 키가 없어도 된다.
		{Enabled: true, Provider: ProviderOpenAICompatible, BaseURL: "http://vllm.internal:8000/v1", Model: "qwen3"},
		// 꺼 두면 키가 없어도 저장할 수 있다.
		{Enabled: false, Provider: ProviderUpstage},
	}
	for _, s := range ok {
		assert.NoError(t, ValidateSettings(s), "%+v", s)
	}

	bad := map[string]Settings{
		"모르는 공급자":              {Enabled: true, Provider: "acme", APIKey: "k"},
		"기본 모델이 없는 공급자의 모델 누락": {Enabled: true, Provider: ProviderOpenAI, APIKey: "k"},
		"켰는데 키가 없다":            {Enabled: true, Provider: ProviderUpstage},
		"직접 입력은 주소가 필요하다":      {Enabled: true, Provider: ProviderOpenAICompatible, Model: "m"},
		"주소는 http(s) 다":        {Enabled: true, Provider: ProviderOpenAICompatible, BaseURL: "file:///etc/passwd", Model: "m"},
		"주소에 비밀번호를 싣지 않는다":     {Enabled: true, Provider: ProviderOpenAICompatible, BaseURL: "https://u:p@llm.internal/v1", Model: "m"},
		"모델 이름이 너무 길다":         {Enabled: true, Provider: ProviderUpstage, APIKey: "k", Model: string(make([]byte, 201))},
	}
	for name, s := range bad {
		t.Run(name, func(t *testing.T) {
			assert.True(t, errors.Is(ValidateSettings(s), ErrInvalidSettings))
		})
	}
}

func TestSettings_ReadyAndKeyHint(t *testing.T) {
	assert.True(t, Settings{Enabled: true, Provider: ProviderUpstage, APIKey: "up_abcd1234"}.Ready())
	assert.False(t, Settings{Enabled: false, Provider: ProviderUpstage, APIKey: "up_abcd1234"}.Ready())
	assert.False(t, Settings{Enabled: true, Provider: ProviderUpstage}.Ready())

	assert.Equal(t, "••••1234", Settings{APIKey: "up_abcd1234"}.KeyHint())
	assert.Equal(t, "••••", Settings{APIKey: "abc"}.KeyHint(), "짧은 키는 끝자리도 보이지 않는다")
	assert.Equal(t, "", Settings{}.KeyHint())
}
