package usecase

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cloud-nullus/draft/internal/assistant/domain"
	"github.com/cloud-nullus/draft/internal/assistant/port"
)

// testTimeout 은 연결 테스트 한 번의 상한이다.
const testTimeout = 20 * time.Second

// SettingsView 는 관리 화면이 보는 설정이다 — 키 원문은 없다.
type SettingsView struct {
	Enabled          bool
	Provider         string
	BaseURL          string
	Model            string
	HasKey           bool
	KeyHint          string
	EffectiveBaseURL string
	EffectiveModel   string
	UpdatedAt        *time.Time
	UpdatedBy        string
	Providers        []domain.Provider
}

// UpdateSettings 는 관리 화면이 보낸 설정이다. APIKey 가 nil 이면 저장된 키를 그대로 쓰고(공급자가 같을 때),
// 빈 문자열이면 지운다.
type UpdateSettings struct {
	Enabled  bool
	Provider string
	BaseURL  string
	Model    string
	APIKey   *string
}

// TestResult 는 연결 테스트 결과다.
type TestResult struct {
	OK        bool
	Model     string
	Error     string
	LatencyMs int64
}

// SettingsService 는 조직의 어시스턴트 설정 — 공급자 · 모델 · 키 — 을 관리한다.
type SettingsService struct {
	repo   port.SettingsRepository
	models port.LLMFactory
	now    func() time.Time
}

func NewSettingsService(repo port.SettingsRepository, models port.LLMFactory) *SettingsService {
	return &SettingsService{repo: repo, models: models, now: time.Now}
}

func (s *SettingsService) Get(ctx context.Context, orgID string) (SettingsView, error) {
	cur, err := s.repo.Get(ctx, orgID)
	if err != nil {
		return SettingsView{}, err
	}
	if cur == nil {
		cur = &domain.Settings{OrgID: orgID, Provider: domain.ProviderUpstage}
	}
	return view(*cur), nil
}

// Update 는 설정을 검증해 저장한다.
func (s *SettingsService) Update(ctx context.Context, orgID, actor string, in UpdateSettings) (SettingsView, error) {
	next, err := s.merge(ctx, orgID, in)
	if err != nil {
		return SettingsView{}, err
	}
	next.UpdatedAt, next.UpdatedBy = s.now(), actor
	if err := domain.ValidateSettings(next); err != nil {
		return SettingsView{}, err
	}
	if err := s.repo.Save(ctx, next); err != nil {
		return SettingsView{}, err
	}
	return view(next), nil
}

func (s *SettingsService) Delete(ctx context.Context, orgID string) error {
	return s.repo.Delete(ctx, orgID)
}

// Test 는 보낸 설정(키를 안 보내면 저장된 키)으로 모델에 한 번 물어 본다. 아무것도 저장하지 않는다. 설정이
// 맞지 않으면 오류, 모델이 실패하면 OK=false 와 사유다 — 사유에서 키는 가린다.
func (s *SettingsService) Test(ctx context.Context, orgID string, in UpdateSettings) (TestResult, error) {
	next, err := s.merge(ctx, orgID, in)
	if err != nil {
		return TestResult{}, err
	}
	next.Enabled = true
	if err := domain.ValidateSettings(next); err != nil {
		return TestResult{}, err
	}
	res := TestResult{Model: next.EffectiveModel()}
	llm, err := s.models.New(next)
	if err != nil {
		res.Error = err.Error()
		return res, nil
	}
	ctx, cancel := context.WithTimeout(ctx, testTimeout)
	defer cancel()
	started := s.now()
	_, err = llm.Complete(ctx, port.CompletionRequest{Messages: []domain.Message{
		{Role: domain.RoleSystem, Content: "Reply with OK."},
		{Role: domain.RoleUser, Content: "ping"},
	}})
	res.LatencyMs = s.now().Sub(started).Milliseconds()
	if err != nil {
		msg := err.Error()
		if next.APIKey != "" {
			msg = strings.ReplaceAll(msg, next.APIKey, "[가림]")
		}
		res.Error = msg
		return res, nil
	}
	res.OK = true
	return res, nil
}

// merge 는 저장된 설정 위에 보낸 값을 얹는다. 키는 보낸 것이 있으면 그것, 없으면 같은 공급자의 저장된 키다 —
// 공급자를 바꾸면 이전 공급자의 키는 쓰지 않는다.
func (s *SettingsService) merge(ctx context.Context, orgID string, in UpdateSettings) (domain.Settings, error) {
	if strings.TrimSpace(orgID) == "" {
		return domain.Settings{}, fmt.Errorf("%w: 조직이 필요하다", domain.ErrInvalidSettings)
	}
	cur, err := s.repo.Get(ctx, orgID)
	if err != nil {
		return domain.Settings{}, err
	}
	next := domain.Settings{
		OrgID:    orgID,
		Enabled:  in.Enabled,
		Provider: strings.TrimSpace(in.Provider),
		BaseURL:  strings.TrimSpace(in.BaseURL),
		Model:    strings.TrimSpace(in.Model),
	}
	switch {
	case in.APIKey != nil:
		next.APIKey = strings.TrimSpace(*in.APIKey)
	case cur != nil && cur.Provider == next.Provider:
		next.APIKey = cur.APIKey
	}
	return next, nil
}

func view(s domain.Settings) SettingsView {
	v := SettingsView{
		Enabled: s.Enabled, Provider: s.Provider, BaseURL: s.BaseURL, Model: s.Model,
		HasKey: s.HasKey(), KeyHint: s.KeyHint(),
		EffectiveBaseURL: s.EffectiveBaseURL(), EffectiveModel: s.EffectiveModel(),
		UpdatedBy: s.UpdatedBy, Providers: domain.Providers(),
	}
	if !s.UpdatedAt.IsZero() {
		t := s.UpdatedAt
		v.UpdatedAt = &t
	}
	return v
}
