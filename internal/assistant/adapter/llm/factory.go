package llm

import (
	"fmt"

	"github.com/cloud-nullus/draft/internal/assistant/domain"
	"github.com/cloud-nullus/draft/internal/assistant/port"
)

// Factory 는 설정의 공급자 프로토콜에 맞는 클라이언트를 만든다.
type Factory struct{}

var _ port.LLMFactory = Factory{}

func (Factory) New(s domain.Settings) (port.LLM, error) {
	p, ok := domain.ProviderByID(s.Provider)
	if !ok {
		return nil, fmt.Errorf("모르는 공급자 %q", s.Provider)
	}
	cfg := Config{BaseURL: s.EffectiveBaseURL(), APIKey: s.APIKey, Model: s.EffectiveModel()}
	switch p.Protocol {
	case domain.ProtocolAnthropic:
		return NewAnthropic(cfg), nil
	case domain.ProtocolOpenAI:
		return New(cfg), nil
	default:
		return nil, fmt.Errorf("공급자 %q 의 프로토콜 %q 를 모른다", p.ID, p.Protocol)
	}
}
