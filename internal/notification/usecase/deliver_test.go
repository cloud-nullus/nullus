package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/notification/domain"
	"github.com/cloud-nullus/draft/internal/notification/port"
	sharednotify "github.com/cloud-nullus/draft/internal/shared/notification"
)

func channel(id, org string, typ domain.ChannelType, active bool, events ...sharednotify.EventType) *domain.Channel {
	return &domain.Channel{ID: id, OrgID: org, Name: "채널 " + id, Type: typ, Active: active, Events: events}
}

type harness struct {
	channels   *fakeChannelRepo
	deliveries *fakeDeliveryRepo
	senders    map[domain.ChannelType]*fakeSender
	sleeps     *recordSleeps
	deliverer  *Deliverer
}

func newHarness(chs ...*domain.Channel) *harness {
	h := &harness{
		channels:   newFakeChannelRepo(chs...),
		deliveries: &fakeDeliveryRepo{},
		senders: map[domain.ChannelType]*fakeSender{
			domain.ChannelTelegram: {}, domain.ChannelEmail: {}, domain.ChannelSlack: {},
		},
		sleeps: &recordSleeps{},
	}
	senders := map[domain.ChannelType]port.Sender{}
	for typ, s := range h.senders {
		senders[typ] = s
	}
	h.deliverer = NewDeliverer(senders, h.deliveries)
	h.deliverer.sleep = h.sleeps.sleep
	return h
}

func backupFailed(org string) sharednotify.Event {
	return sharednotify.Event{
		Type: sharednotify.EventBackupFailed, OrgID: org, Severity: sharednotify.SeverityCritical,
		Title: "백업 실패", Summary: "목적지에 쓸 수 없습니다",
	}
}

// 이벤트는 그 조직의, 켜져 있고, 그 이벤트를 구독한 채널로만 간다.
func TestDispatcher_Publish_FansOutToSubscribedActiveChannelsOfOrg(t *testing.T) {
	h := newHarness(
		channel("tg", "org-1", domain.ChannelTelegram, true, sharednotify.EventBackupFailed),
		channel("mail", "org-1", domain.ChannelEmail, true, sharednotify.EventBackupFailed, sharednotify.EventBackupPartial),
		channel("slack-off", "org-1", domain.ChannelSlack, false, sharednotify.EventBackupFailed),
		channel("slack-success", "org-1", domain.ChannelSlack, true, sharednotify.EventBackupSucceeded),
		channel("other-org", "org-2", domain.ChannelSlack, true, sharednotify.EventBackupFailed),
	)
	d := NewDispatcher(h.channels, h.deliverer)

	require.NoError(t, d.Publish(context.Background(), backupFailed("org-1")))

	assert.Len(t, h.senders[domain.ChannelTelegram].sent, 1)
	assert.Len(t, h.senders[domain.ChannelEmail].sent, 1)
	assert.Empty(t, h.senders[domain.ChannelSlack].sent)
	assert.Equal(t, "백업 실패", h.senders[domain.ChannelTelegram].sent[0].Message.Title)

	require.Len(t, h.deliveries.deliveries, 2)
	for _, d := range h.deliveries.deliveries {
		assert.Equal(t, domain.DeliverySent, d.Status)
		assert.Equal(t, "org-1", d.OrgID)
		assert.Equal(t, sharednotify.EventBackupFailed, d.Event)
		assert.Equal(t, 1, d.Attempts)
	}
}

// 한 채널이 실패해도 나머지는 보낸다. 실패는 이력에 남고 Publish 가 모아 돌려준다.
func TestDispatcher_Publish_OneChannelFailureDoesNotStopOthers(t *testing.T) {
	h := newHarness(
		channel("tg", "org-1", domain.ChannelTelegram, true, sharednotify.EventBackupFailed),
		channel("mail", "org-1", domain.ChannelEmail, true, sharednotify.EventBackupFailed),
	)
	h.senders[domain.ChannelTelegram].errs = []error{&port.SendError{Err: errors.New("chat not found")}}
	d := NewDispatcher(h.channels, h.deliverer)

	err := d.Publish(context.Background(), backupFailed("org-1"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "chat not found")
	assert.Len(t, h.senders[domain.ChannelEmail].sent, 1)

	statuses := map[string]domain.DeliveryStatus{}
	for _, d := range h.deliveries.deliveries {
		statuses[d.ChannelID] = d.Status
	}
	assert.Equal(t, map[string]domain.DeliveryStatus{"tg": domain.DeliveryFailed, "mail": domain.DeliverySent}, statuses)
}

func TestDispatcher_Publish_NoChannelsIsNotAnError(t *testing.T) {
	h := newHarness()
	assert.NoError(t, NewDispatcher(h.channels, h.deliverer).Publish(context.Background(), backupFailed("org-1")))
	assert.Empty(t, h.deliveries.deliveries)
}

// 일시적인 실패(네트워크 · 5xx · 429)는 물러섰다가 다시 보낸다. 서버가 기다릴 시간을 주면 따른다.
func TestDeliverer_RetriesRetryableFailures(t *testing.T) {
	h := newHarness(channel("tg", "org-1", domain.ChannelTelegram, true, sharednotify.EventBackupFailed))
	h.senders[domain.ChannelTelegram].errs = []error{
		&port.SendError{Err: errors.New("dial timeout"), Retryable: true},
		&port.SendError{Err: errors.New("too many requests"), Retryable: true, RetryAfter: 3 * time.Second},
	}
	ch, _ := h.channels.Get(context.Background(), "org-1", "tg")

	d := h.deliverer.Deliver(context.Background(), ch, domain.NewMessage(backupFailed("org-1")))

	assert.Equal(t, domain.DeliverySent, d.Status)
	assert.Equal(t, 3, d.Attempts)
	assert.Equal(t, []time.Duration{time.Second, 3 * time.Second}, h.sleeps.waits)
}

// 설정이 틀린 실패는 다시 보내도 같다 — 바로 실패로 적는다.
func TestDeliverer_DoesNotRetryPermanentFailures(t *testing.T) {
	h := newHarness(channel("slack", "org-1", domain.ChannelSlack, true, sharednotify.EventBackupFailed))
	h.senders[domain.ChannelSlack].errs = []error{&port.SendError{Err: errors.New("no_service")}}
	ch, _ := h.channels.Get(context.Background(), "org-1", "slack")

	d := h.deliverer.Deliver(context.Background(), ch, domain.NewMessage(backupFailed("org-1")))

	assert.Equal(t, domain.DeliveryFailed, d.Status)
	assert.Equal(t, 1, d.Attempts)
	assert.Equal(t, "no_service", d.Error)
	assert.Empty(t, h.sleeps.waits)
}

func TestDeliverer_GivesUpAfterMaxAttemptsAndCapsWait(t *testing.T) {
	h := newHarness(channel("tg", "org-1", domain.ChannelTelegram, true, sharednotify.EventBackupFailed))
	h.senders[domain.ChannelTelegram].errs = []error{
		&port.SendError{Err: errors.New("503"), Retryable: true, RetryAfter: time.Hour},
		&port.SendError{Err: errors.New("503"), Retryable: true},
		&port.SendError{Err: errors.New("503 again"), Retryable: true},
	}
	ch, _ := h.channels.Get(context.Background(), "org-1", "tg")

	d := h.deliverer.Deliver(context.Background(), ch, domain.NewMessage(backupFailed("org-1")))

	assert.Equal(t, domain.DeliveryFailed, d.Status)
	assert.Equal(t, maxAttempts, d.Attempts)
	assert.Equal(t, "503 again", d.Error)
	assert.Equal(t, []time.Duration{maxRetryWait, 2 * time.Second}, h.sleeps.waits, "서버가 한 시간을 달라 해도 백업을 붙들지 않는다")
}

func TestDeliverer_UnsupportedChannelTypeFails(t *testing.T) {
	h := newHarness()
	ch := channel("x", "org-1", "pager", true, sharednotify.EventBackupFailed)
	d := h.deliverer.Deliver(context.Background(), ch, domain.NewMessage(backupFailed("org-1")))
	assert.Equal(t, domain.DeliveryFailed, d.Status)
	assert.Contains(t, d.Error, "pager")
}

// 테스트 전송은 꺼진 채널도 보낸다 — 켜기 전에 확인하는 버튼이다.
func TestSendTest_SendsEvenToInactiveChannelAndRecords(t *testing.T) {
	h := newHarness(channel("tg", "org-1", domain.ChannelTelegram, false, sharednotify.EventBackupFailed))
	st := NewSendTest(h.channels, h.deliverer)

	d, err := st.Execute(context.Background(), "org-1", "tg")
	require.NoError(t, err)
	assert.Equal(t, domain.DeliverySent, d.Status)
	assert.Equal(t, domain.EventTest, d.Event)
	require.Len(t, h.senders[domain.ChannelTelegram].sent, 1)
	assert.Equal(t, domain.EventTest, h.senders[domain.ChannelTelegram].sent[0].Message.Event)
	assert.Len(t, h.deliveries.deliveries, 1)

	_, err = st.Execute(context.Background(), "org-2", "tg")
	assert.ErrorIs(t, err, domain.ErrChannelNotFound)
}

func TestListDeliveries_ClampsLimit(t *testing.T) {
	h := newHarness()
	for i := 0; i < 5; i++ {
		_ = h.deliveries.Record(context.Background(), &domain.Delivery{OrgID: "org-1", Title: "x"})
	}
	ld := NewListDeliveries(h.deliveries)
	items, err := ld.Execute(context.Background(), "org-1", 0)
	require.NoError(t, err)
	assert.Len(t, items, 5)
	items, err = ld.Execute(context.Background(), "org-1", 2)
	require.NoError(t, err)
	assert.Len(t, items, 2)
}
