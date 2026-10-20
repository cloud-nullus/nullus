package domain

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

// Protocol 은 모델 API 의 모양이다. 벤더는 여럿이어도 모양은 둘이다.
type Protocol string

const (
	// ProtocolOpenAI 는 OpenAI 호환 Chat Completions 다(Upstage · OpenAI · Gemini 의 OpenAI 호환 엔드포인트 · vLLM 등).
	ProtocolOpenAI Protocol = "openai"
	// ProtocolAnthropic 은 Anthropic Messages API 다.
	ProtocolAnthropic Protocol = "anthropic"
)

// 공급자 ID.
const (
	ProviderUpstage          = "upstage"
	ProviderOpenAI           = "openai"
	ProviderAnthropic        = "anthropic"
	ProviderGemini           = "gemini"
	ProviderOpenAICompatible = "openai-compatible"
)

// MaxModelChars 는 모델 이름의 길이 상한이다.
const MaxModelChars = 200

// Provider 는 고를 수 있는 모델 공급자다. 기본 주소 · 모델이 비어 있으면 관리자가 넣어야 한다.
type Provider struct {
	ID             string   `json:"id"`
	Protocol       Protocol `json:"protocol"`
	DefaultBaseURL string   `json:"default_base_url,omitempty"`
	DefaultModel   string   `json:"default_model,omitempty"`
	// KeyRequired 가 거짓이면 키 없이도 켤 수 있다(사내 서버).
	KeyRequired bool `json:"key_required"`
}

var providers = []Provider{
	{ID: ProviderUpstage, Protocol: ProtocolOpenAI, DefaultBaseURL: "https://api.upstage.ai/v1", DefaultModel: "solar-pro4", KeyRequired: true},
	{ID: ProviderOpenAI, Protocol: ProtocolOpenAI, DefaultBaseURL: "https://api.openai.com/v1", KeyRequired: true},
	{ID: ProviderAnthropic, Protocol: ProtocolAnthropic, DefaultBaseURL: "https://api.anthropic.com", DefaultModel: "claude-sonnet-5-5", KeyRequired: true},
	{ID: ProviderGemini, Protocol: ProtocolOpenAI, DefaultBaseURL: "https://generativelanguage.googleapis.com/v1beta/openai", KeyRequired: true},
	{ID: ProviderOpenAICompatible, Protocol: ProtocolOpenAI, KeyRequired: false},
}

// Providers 는 고를 수 있는 공급자다.
func Providers() []Provider { return append([]Provider(nil), providers...) }

// ProviderByID 는 공급자를 찾는다.
func ProviderByID(id string) (Provider, bool) {
	for _, p := range providers {
		if p.ID == id {
			return p, true
		}
	}
	return Provider{}, false
}

var (
	// ErrInvalidSettings 는 관리자가 넣은 설정이 맞지 않는다는 뜻이다.
	ErrInvalidSettings = errors.New("assistant: invalid settings")
	// ErrKeyStorageUnavailable 은 API 키를 안전하게 저장할 수 없다는 뜻이다(ENCRYPTION_KEY 없음).
	ErrKeyStorageUnavailable = errors.New("assistant: key storage unavailable")
)

// Settings 는 조직의 어시스턴트 설정이다. APIKey 는 저장소에서 복호화한 원문이다 — 응답에는 KeyHint 만 싣는다.
type Settings struct {
	OrgID     string
	Enabled   bool
	Provider  string
	BaseURL   string
	Model     string
	APIKey    string
	UpdatedAt time.Time
	UpdatedBy string
}

// EffectiveBaseURL · EffectiveModel 은 비어 있으면 공급자 기본값이다.
func (s Settings) EffectiveBaseURL() string {
	if v := strings.TrimSpace(s.BaseURL); v != "" {
		return strings.TrimRight(v, "/")
	}
	p, _ := ProviderByID(s.Provider)
	return p.DefaultBaseURL
}

func (s Settings) EffectiveModel() string {
	if v := strings.TrimSpace(s.Model); v != "" {
		return v
	}
	p, _ := ProviderByID(s.Provider)
	return p.DefaultModel
}

// HasKey 는 키가 저장돼 있는지다.
func (s Settings) HasKey() bool { return s.APIKey != "" }

// KeyHint 는 화면에 보일 키 표시다 — 끝 네 자리만. 8자보다 짧으면 끝자리도 보이지 않는다.
func (s Settings) KeyHint() string {
	if s.APIKey == "" {
		return ""
	}
	r := []rune(s.APIKey)
	if len(r) < 8 {
		return "••••"
	}
	return "••••" + string(r[len(r)-4:])
}

// Ready 는 대화에 쓸 수 있는 설정인지다.
func (s Settings) Ready() bool {
	return s.Enabled && ValidateSettings(s) == nil
}

// ValidateSettings 는 설정을 검증한다. 꺼진 설정은 공급자만 맞으면 저장할 수 있다.
func ValidateSettings(s Settings) error {
	p, ok := ProviderByID(s.Provider)
	if !ok {
		return fmt.Errorf("%w: 모르는 공급자 %q", ErrInvalidSettings, s.Provider)
	}
	if utf8.RuneCountInString(s.Model) > MaxModelChars {
		return fmt.Errorf("%w: 모델 이름은 %d자까지다", ErrInvalidSettings, MaxModelChars)
	}
	if raw := strings.TrimSpace(s.BaseURL); raw != "" {
		u, err := url.Parse(raw)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return fmt.Errorf("%w: 주소는 http(s) URL 이어야 한다", ErrInvalidSettings)
		}
		if u.User != nil {
			return fmt.Errorf("%w: 주소에 계정 정보를 넣지 않는다 — 키 칸을 쓴다", ErrInvalidSettings)
		}
	}
	if !s.Enabled {
		return nil
	}
	if s.EffectiveBaseURL() == "" {
		return fmt.Errorf("%w: 이 공급자는 주소가 필요하다", ErrInvalidSettings)
	}
	if s.EffectiveModel() == "" {
		return fmt.Errorf("%w: 이 공급자는 모델 이름이 필요하다", ErrInvalidSettings)
	}
	if p.KeyRequired && !s.HasKey() {
		return fmt.Errorf("%w: API 키가 필요하다", ErrInvalidSettings)
	}
	return nil
}
