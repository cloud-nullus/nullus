// Package assistant 는 AI 어시스턴트 모듈(nullus-plan#94)이다 — Nullus MCP 도구를 Upstage Solar(또는 OpenAI 호환
// 모델)에 물려, 화면에서 자연어로 묻고 답한다. 설계: docs/11_기능설계/Nullus_AI_어시스턴트_설계.md.
package assistant

import (
	"strings"

	"github.com/cloud-nullus/draft/internal/assistant/adapter/llm"
	"github.com/cloud-nullus/draft/internal/assistant/adapter/mcphost"
	"github.com/cloud-nullus/draft/internal/assistant/port"
	"github.com/cloud-nullus/draft/internal/assistant/usecase"
)

// 환경 변수 이름.
const (
	EnvAPIKey  = "NULLUS_ASSISTANT_API_KEY"
	EnvBaseURL = "NULLUS_ASSISTANT_BASE_URL"
	EnvModel   = "NULLUS_ASSISTANT_MODEL"
)

type Config struct {
	APIKey  string
	BaseURL string
	Model   string
	// ServerURL 은 도구가 부를 API 자신(루프백)이다.
	ServerURL string
	Version   string
}

func ConfigFromEnv(getenv func(string) string, serverURL, version string) Config {
	return Config{
		APIKey:    strings.TrimSpace(getenv(EnvAPIKey)),
		BaseURL:   strings.TrimSpace(getenv(EnvBaseURL)),
		Model:     strings.TrimSpace(getenv(EnvModel)),
		ServerURL: serverURL,
		Version:   version,
	}
}

// Module 은 조립된 어시스턴트다. HTTP 핸들러는 main 이 Chat 으로 만든다(라우트 배선 검사가 main 에서 본다).
type Module struct {
	Chat     *usecase.Chat
	Provider string
	Model    string
}

// NewModule 은 모듈을 조립한다. 키가 없으면 꺼진 채로 만든다 — 상태 API 가 enabled=false 를 돌려주고
// 대화 API 는 503 이다.
func NewModule(cfg Config) *Module {
	m := &Module{}
	var model port.LLM
	if cfg.APIKey != "" {
		client := llm.New(llm.Config{BaseURL: cfg.BaseURL, APIKey: cfg.APIKey, Model: cfg.Model})
		model, m.Model = client, client.Model()
		m.Provider = "upstage"
		if cfg.BaseURL != "" && !strings.Contains(cfg.BaseURL, "upstage.ai") {
			m.Provider = "openai-compatible"
		}
	}
	m.Chat = usecase.NewChat(model, mcphost.NewOpener(cfg.ServerURL, cfg.Version))
	return m
}

// Enabled 는 모델이 설정됐는지다.
func (m *Module) Enabled() bool { return m.Chat.Enabled() }
