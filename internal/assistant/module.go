// Package assistant 는 AI 어시스턴트 모듈(nullus-plan#94)이다 — Nullus MCP 조회 도구를 조직이 고른 모델
// (Upstage · OpenAI · Anthropic · Gemini · OpenAI 호환 사내 서버)에 물려, 화면에서 자연어로 묻고 답한다.
// 공급자 · 모델 · API 키는 관리 화면에서 조직마다 등록한다. 설계: docs/11_기능설계/Nullus_AI_어시스턴트_설계.md.
package assistant

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cloud-nullus/draft/internal/assistant/adapter/llm"
	"github.com/cloud-nullus/draft/internal/assistant/adapter/mcphost"
	"github.com/cloud-nullus/draft/internal/assistant/adapter/repository"
	"github.com/cloud-nullus/draft/internal/assistant/port"
	"github.com/cloud-nullus/draft/internal/assistant/usecase"
)

// Deps 는 모듈을 조립하는 데 필요한 것이다.
type Deps struct {
	// Settings 가 없으면 Pool 과 EncryptionKey 로 Postgres 저장소를 만든다.
	Settings      port.SettingsRepository
	Pool          *pgxpool.Pool
	EncryptionKey []byte
	// ServerURL 은 도구가 부를 API 자신(루프백)이다.
	ServerURL string
	Version   string
}

// Module 은 조립된 어시스턴트다. HTTP 핸들러는 main 이 만든다(라우트 배선 검사가 main 에서 본다).
type Module struct {
	Chat     *usecase.Chat
	Settings *usecase.SettingsService
}

func New(d Deps) *Module {
	settings := d.Settings
	if settings == nil {
		settings = repository.NewPostgresSettingsRepository(d.Pool, d.EncryptionKey)
	}
	models := llm.Factory{}
	return &Module{
		Chat:     usecase.NewChat(settings, models, mcphost.NewOpener(d.ServerURL, d.Version)),
		Settings: usecase.NewSettingsService(settings, models),
	}
}
