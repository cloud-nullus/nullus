// Package usecase 는 알림 채널 관리와 발송이다.
package usecase

import (
	"context"
	"errors"
	"strings"

	"github.com/cloud-nullus/draft/internal/notification/domain"
	"github.com/cloud-nullus/draft/internal/notification/port"
	sharednotify "github.com/cloud-nullus/draft/internal/shared/notification"
)

// ErrOrgRequired 는 조직 없이 채널을 다루려 한 경우다 — 채널은 조직마다 따로다.
var ErrOrgRequired = errors.New("organization is required")

// ChannelService 는 조직의 알림 채널을 만들고 고치고 지운다.
type ChannelService struct {
	repo port.ChannelRepository
}

func NewChannelService(repo port.ChannelRepository) *ChannelService {
	return &ChannelService{repo: repo}
}

// CreateChannelInput 은 새 채널이다. Active 가 nil 이면 켠다.
type CreateChannelInput struct {
	OrgID    string
	Name     string
	Type     domain.ChannelType
	Settings domain.Settings
	Secret   string
	Events   []sharednotify.EventType
	Active   *bool
}

func (s *ChannelService) Create(ctx context.Context, in CreateChannelInput) (*domain.Channel, error) {
	if strings.TrimSpace(in.OrgID) == "" {
		return nil, ErrOrgRequired
	}
	active := true
	if in.Active != nil {
		active = *in.Active
	}
	ch := &domain.Channel{
		OrgID: in.OrgID, Name: in.Name, Type: in.Type, Settings: in.Settings,
		Secret: in.Secret, Events: in.Events, Active: active,
	}
	if err := ch.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, ch); err != nil {
		return nil, err
	}
	return ch, nil
}

func (s *ChannelService) List(ctx context.Context, orgID string) ([]*domain.Channel, error) {
	if strings.TrimSpace(orgID) == "" {
		return nil, ErrOrgRequired
	}
	return s.repo.List(ctx, orgID)
}

func (s *ChannelService) Get(ctx context.Context, orgID, id string) (*domain.Channel, error) {
	return s.repo.Get(ctx, orgID, id)
}

// Update 는 바꿀 칸만 적용한다. 비밀을 비워 보내면 기존 값을 지킨다.
func (s *ChannelService) Update(ctx context.Context, orgID, id string, u domain.ChannelUpdate) (*domain.Channel, error) {
	ch, err := s.repo.Get(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	if err := ch.ApplyUpdate(u); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, ch); err != nil {
		return nil, err
	}
	return ch, nil
}

func (s *ChannelService) Delete(ctx context.Context, orgID, id string) error {
	return s.repo.Delete(ctx, orgID, id)
}
