package assistant

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

// 키가 없으면 꺼진다 — 폐쇄망 · 미동의 고객은 아무것도 바뀌지 않는다.
func TestNewModule_DisabledWithoutKey(t *testing.T) {
	m := NewModule(ConfigFromEnv(env(nil), "http://127.0.0.1:8090", "dev"))
	assert.False(t, m.Enabled())
	assert.NotNil(t, m.Chat)
}

func TestNewModule_UpstageByDefault(t *testing.T) {
	m := NewModule(ConfigFromEnv(env(map[string]string{"NULLUS_ASSISTANT_API_KEY": " up_key "}), "http://127.0.0.1:8090", "dev"))
	assert.True(t, m.Enabled())
	assert.Equal(t, "upstage", m.Provider)
	assert.Equal(t, "solar-pro4", m.Model)
}

// base URL 을 바꾸면 사내 OpenAI 호환 서버다.
func TestNewModule_OpenAICompatibleBaseURL(t *testing.T) {
	m := NewModule(ConfigFromEnv(env(map[string]string{
		"NULLUS_ASSISTANT_API_KEY":  "k",
		"NULLUS_ASSISTANT_BASE_URL": "http://vllm.internal:8000/v1",
		"NULLUS_ASSISTANT_MODEL":    "qwen3",
	}), "http://127.0.0.1:8090", "dev"))
	assert.Equal(t, "openai-compatible", m.Provider)
	assert.Equal(t, "qwen3", m.Model)
}
