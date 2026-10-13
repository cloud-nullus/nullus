package repository

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/cloud-nullus/draft/internal/notification/domain"
	"github.com/cloud-nullus/draft/internal/notification/port"
)

// MemoryChannelRepository 는 DB 없이 도는 테스트용 채널 저장소다.
type MemoryChannelRepository struct {
	mu       sync.Mutex
	seq      int
	channels map[string]domain.Channel
}

func NewMemoryChannelRepository() *MemoryChannelRepository {
	return &MemoryChannelRepository{channels: map[string]domain.Channel{}}
}

var _ port.ChannelRepository = (*MemoryChannelRepository)(nil)

func (r *MemoryChannelRepository) Create(_ context.Context, ch *domain.Channel) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seq++
	ch.ID = fmt.Sprintf("00000000-0000-0000-0000-%012d", r.seq)
	ch.CreatedAt = time.Now().UTC()
	ch.UpdatedAt = ch.CreatedAt
	r.channels[ch.ID] = *ch
	return nil
}

func (r *MemoryChannelRepository) Get(_ context.Context, orgID, id string) (*domain.Channel, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ch, ok := r.channels[id]
	if !ok || ch.OrgID != orgID {
		return nil, domain.ErrChannelNotFound
	}
	return &ch, nil
}

func (r *MemoryChannelRepository) List(_ context.Context, orgID string) ([]*domain.Channel, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.Channel
	for _, ch := range r.channels {
		if ch.OrgID == orgID {
			c := ch
			out = append(out, &c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (r *MemoryChannelRepository) Update(_ context.Context, ch *domain.Channel) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	old, ok := r.channels[ch.ID]
	if !ok || old.OrgID != ch.OrgID {
		return domain.ErrChannelNotFound
	}
	ch.UpdatedAt = time.Now().UTC()
	r.channels[ch.ID] = *ch
	return nil
}

func (r *MemoryChannelRepository) Delete(_ context.Context, orgID, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	ch, ok := r.channels[id]
	if !ok || ch.OrgID != orgID {
		return domain.ErrChannelNotFound
	}
	delete(r.channels, id)
	return nil
}

// MemoryDeliveryRepository 는 DB 없이 도는 테스트용 발송 이력이다.
type MemoryDeliveryRepository struct {
	mu    sync.Mutex
	items []domain.Delivery
}

func NewMemoryDeliveryRepository() *MemoryDeliveryRepository { return &MemoryDeliveryRepository{} }

var _ port.DeliveryRepository = (*MemoryDeliveryRepository)(nil)

func (r *MemoryDeliveryRepository) Record(_ context.Context, d *domain.Delivery) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	d.ID = fmt.Sprintf("dlv-%d", len(r.items)+1)
	r.items = append(r.items, *d)
	return nil
}

func (r *MemoryDeliveryRepository) List(_ context.Context, orgID string, limit int) ([]*domain.Delivery, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.Delivery
	for i := len(r.items) - 1; i >= 0 && len(out) < limit; i-- {
		if r.items[i].OrgID == orgID {
			d := r.items[i]
			out = append(out, &d)
		}
	}
	return out, nil
}
