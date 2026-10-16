package usecase

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/cloud-nullus/draft/internal/notification/domain"
)

type fakeChannelRepo struct {
	mu       sync.Mutex
	channels map[string]*domain.Channel
	seq      int
}

func newFakeChannelRepo(chs ...*domain.Channel) *fakeChannelRepo {
	r := &fakeChannelRepo{channels: map[string]*domain.Channel{}}
	for _, ch := range chs {
		_ = r.Create(context.Background(), ch)
	}
	return r
}

func (r *fakeChannelRepo) Create(_ context.Context, ch *domain.Channel) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seq++
	if ch.ID == "" {
		ch.ID = fmt.Sprintf("ch-%d", r.seq)
	}
	ch.CreatedAt = time.Date(2026, 10, 13, 0, 0, r.seq, 0, time.UTC)
	copied := *ch
	r.channels[ch.ID] = &copied
	return nil
}

func (r *fakeChannelRepo) Get(_ context.Context, orgID, id string) (*domain.Channel, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ch, ok := r.channels[id]
	if !ok || ch.OrgID != orgID {
		return nil, domain.ErrChannelNotFound
	}
	copied := *ch
	return &copied, nil
}

func (r *fakeChannelRepo) List(_ context.Context, orgID string) ([]*domain.Channel, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.Channel
	for _, ch := range r.channels {
		if ch.OrgID == orgID {
			copied := *ch
			out = append(out, &copied)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (r *fakeChannelRepo) Update(_ context.Context, ch *domain.Channel) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.channels[ch.ID]; !ok {
		return domain.ErrChannelNotFound
	}
	copied := *ch
	r.channels[ch.ID] = &copied
	return nil
}

func (r *fakeChannelRepo) Delete(_ context.Context, orgID, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	ch, ok := r.channels[id]
	if !ok || ch.OrgID != orgID {
		return domain.ErrChannelNotFound
	}
	delete(r.channels, id)
	return nil
}

type fakeDeliveryRepo struct {
	mu         sync.Mutex
	deliveries []*domain.Delivery
}

func (r *fakeDeliveryRepo) Record(ctx context.Context, d *domain.Delivery) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	copied := *d
	r.deliveries = append(r.deliveries, &copied)
	return nil
}

func (r *fakeDeliveryRepo) List(_ context.Context, orgID string, limit int) ([]*domain.Delivery, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.Delivery
	for i := len(r.deliveries) - 1; i >= 0 && len(out) < limit; i-- {
		if r.deliveries[i].OrgID == orgID {
			out = append(out, r.deliveries[i])
		}
	}
	return out, nil
}

// fakeSender 는 보낸 것을 적고, 차례대로 준비된 오류를 돌려준다.
type fakeSender struct {
	mu     sync.Mutex
	sent   []sentMessage
	errs   []error
	called int
	// block 이 있으면 닫힐 때까지(또는 ctx 가 끝날 때까지) 기다린다 — 느린 채널.
	block chan struct{}
}

type sentMessage struct {
	ChannelID string
	Message   domain.Message
}

func (s *fakeSender) Send(ctx context.Context, ch *domain.Channel, msg domain.Message) error {
	if s.block != nil {
		select {
		case <-s.block:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.called++
	if len(s.errs) > 0 {
		err := s.errs[0]
		s.errs = s.errs[1:]
		if err != nil {
			return err
		}
	}
	s.sent = append(s.sent, sentMessage{ChannelID: ch.ID, Message: msg})
	return nil
}

// recordSleeps 는 재시도 대기를 실제로 기다리지 않고 적는다.
type recordSleeps struct{ waits []time.Duration }

func (r *recordSleeps) sleep(ctx context.Context, d time.Duration) error {
	r.waits = append(r.waits, d)
	return ctx.Err()
}
