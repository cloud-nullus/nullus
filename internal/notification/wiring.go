// Package notification 은 알림 채널 모듈이다(nullus-plan#63) — Telegram · 이메일(SMTP) · Slack.
//
// 조직이 채널을 등록하고 받을 이벤트를 고르면, 다른 모듈이 낸 이벤트(sharednotify.Event)가 그 채널로
// 간다. 지금 이벤트를 내는 곳은 백업(B3-3)이다. 설계: docs/11_기능설계/Nullus_알림_채널_설계.md.
package notification

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cloud-nullus/draft/internal/notification/adapter/repository"
	"github.com/cloud-nullus/draft/internal/notification/adapter/sender"
	"github.com/cloud-nullus/draft/internal/notification/domain"
	"github.com/cloud-nullus/draft/internal/notification/port"
	"github.com/cloud-nullus/draft/internal/notification/usecase"
	sharednotify "github.com/cloud-nullus/draft/internal/shared/notification"
)

// Deps 는 모듈을 조립하는 데 필요한 것이다.
type Deps struct {
	Pool *pgxpool.Pool
	// EncryptionKey 는 채널 비밀(봇 토큰 · 웹훅 URL · SMTP 비밀번호)을 암호화하는 키다(ENCRYPTION_KEY).
	EncryptionKey []byte
	// TelegramAPIURL 은 Bot API 주소다. 비면 https://api.telegram.org (NULLUS_TELEGRAM_API_URL).
	TelegramAPIURL string
	Logger         *slog.Logger
}

// Module 은 조립된 알림 모듈이다. 유스케이스는 main 이 관리자 API 핸들러(adapter/handler)를 만들 때
// 쓰고, Publisher 는 다른 모듈이 이벤트를 내는 창구다.
type Module struct {
	Channels   *usecase.ChannelService
	SendTest   *usecase.SendTest
	Deliveries *usecase.ListDeliveries
	Publisher  sharednotify.Publisher
}

func New(ctx context.Context, d Deps) *Module {
	logger := d.Logger
	if logger == nil {
		logger = slog.Default()
	}
	channels := repository.NewPostgresChannelRepository(d.Pool, d.EncryptionKey)
	if len(d.EncryptionKey) != 32 {
		logger.Warn("ENCRYPTION_KEY 가 없어 비밀이 필요한 알림 채널(Telegram · Slack · 인증 SMTP)을 저장할 수 없습니다")
	}
	// 000096 전에 config 에 평문으로 들어간 비밀을 옮겨 암호화한다. 실패해도 API 는 뜬다 — 그 행은
	// 옮길 때까지 평문 그대로 읽힌다.
	if moved, err := channels.MigrateLegacySecrets(ctx); err != nil {
		logger.Warn("알림 채널의 평문 비밀을 옮기지 못했습니다", "error", err)
	} else if moved > 0 {
		logger.Info("알림 채널의 평문 비밀을 암호화해 옮겼습니다", "channels", moved)
	}
	deliveries := repository.NewPostgresDeliveryRepository(d.Pool)
	deliverer := usecase.NewDeliverer(map[domain.ChannelType]port.Sender{
		domain.ChannelTelegram: sender.NewTelegramSender(d.TelegramAPIURL, nil),
		domain.ChannelEmail:    sender.NewEmailSender(),
		domain.ChannelSlack:    sender.NewSlackSender(nil),
	}, deliveries)
	return &Module{
		Channels:   usecase.NewChannelService(channels),
		SendTest:   usecase.NewSendTest(channels, deliverer),
		Deliveries: usecase.NewListDeliveries(deliveries),
		Publisher:  usecase.NewDispatcher(channels, deliverer),
	}
}
