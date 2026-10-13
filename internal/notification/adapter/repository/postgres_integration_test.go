//go:build integration

package repository

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/cloud-nullus/draft/internal/notification/domain"
	sharednotify "github.com/cloud-nullus/draft/internal/shared/notification"
)

var testKey = []byte("0123456789abcdef0123456789abcdef")

const itToken = "123456789:AAHdqTcvCH1vGWJxfSeofSAs0K5PALDsaw0"

func TestNotificationRepositories_Integration(t *testing.T) {
	pool := setupPostgres(t)
	ctx := context.Background()
	channels := NewPostgresChannelRepository(pool, testKey)
	deliveries := NewPostgresDeliveryRepository(pool)

	ch := &domain.Channel{
		OrgID: "org-it", Name: "운영 텔레그램", Type: domain.ChannelTelegram,
		Settings: domain.Settings{ChatID: "-100123"}, Secret: itToken,
		Events: []sharednotify.EventType{sharednotify.EventBackupFailed, sharednotify.EventBackupPartial},
		Active: true,
	}
	require.NoError(t, channels.Create(ctx, ch))
	require.NotEmpty(t, ch.ID)

	t.Run("비밀은 암호화해 저장하고 읽을 때 푼다", func(t *testing.T) {
		var raw, config string
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT secret_encrypted, config::text FROM notification_configs WHERE id = $1`, ch.ID).Scan(&raw, &config))
		assert.NotContains(t, raw, "AAHdqTcv")
		assert.NotContains(t, config, "AAHdqTcv")
		assert.Contains(t, config, `"chat_id": "-100123"`)

		got, err := channels.Get(ctx, "org-it", ch.ID)
		require.NoError(t, err)
		assert.Equal(t, itToken, got.Secret)
		assert.Equal(t, ch.Events, got.Events)
		assert.Equal(t, "운영 텔레그램", got.Name)
		assert.True(t, got.Active)
	})

	t.Run("다른 조직 · 잘못된 ID 는 없는 채널이다", func(t *testing.T) {
		_, err := channels.Get(ctx, "org-other", ch.ID)
		assert.ErrorIs(t, err, domain.ErrChannelNotFound)
		_, err = channels.Get(ctx, "org-it", "not-a-uuid")
		assert.ErrorIs(t, err, domain.ErrChannelNotFound)
		assert.ErrorIs(t, channels.Delete(ctx, "org-other", ch.ID), domain.ErrChannelNotFound)
		list, err := channels.List(ctx, "org-other")
		require.NoError(t, err)
		assert.Empty(t, list)
	})

	t.Run("갱신", func(t *testing.T) {
		ch.Name = "이름 바꿈"
		ch.Active = false
		require.NoError(t, channels.Update(ctx, ch))
		got, err := channels.Get(ctx, "org-it", ch.ID)
		require.NoError(t, err)
		assert.Equal(t, "이름 바꿈", got.Name)
		assert.False(t, got.Active)
		assert.Equal(t, itToken, got.Secret)
	})

	t.Run("발송 이력 — 채널을 지워도 보낼 때 이름으로 남는다", func(t *testing.T) {
		d := &domain.Delivery{
			OrgID: "org-it", ChannelID: ch.ID, ChannelName: "보낼 때 이름", ChannelType: domain.ChannelTelegram,
			Event: sharednotify.EventBackupFailed, Status: domain.DeliveryFailed, Title: "백업 실패",
			Attempts: 3, Error: "HTTP 400: chat not found",
		}
		require.NoError(t, deliveries.Record(ctx, d))
		require.NotEmpty(t, d.ID)

		list, err := deliveries.List(ctx, "org-it", 10)
		require.NoError(t, err)
		require.Len(t, list, 1)
		assert.Equal(t, "이름 바꿈", list[0].ChannelName, "채널이 있으면 지금 이름")
		assert.Equal(t, 3, list[0].Attempts)
		assert.Equal(t, "HTTP 400: chat not found", list[0].Error)
		assert.Equal(t, domain.DeliveryFailed, list[0].Status)

		require.NoError(t, channels.Delete(ctx, "org-it", ch.ID))
		list, err = deliveries.List(ctx, "org-it", 10)
		require.NoError(t, err)
		require.Len(t, list, 1)
		assert.Equal(t, "", list[0].ChannelID)
		assert.Equal(t, "보낼 때 이름", list[0].ChannelName)

		other, err := deliveries.List(ctx, "org-other", 10)
		require.NoError(t, err)
		assert.Empty(t, other)
	})

	t.Run("000096 전의 평문 비밀을 옮겨 암호화한다", func(t *testing.T) {
		var id string
		require.NoError(t, pool.QueryRow(ctx, `
			INSERT INTO notification_configs (org_id, channel, config, events)
			VALUES ('org-legacy', 'slack', '{"webhook_url": "https://hooks.slack.com/services/T/B/LEGACYSECRET", "channel": "#ops"}', ARRAY['backup_failed'])
			RETURNING id::text`).Scan(&id))

		before, err := channels.Get(ctx, "org-legacy", id)
		require.NoError(t, err)
		assert.Equal(t, "https://hooks.slack.com/services/T/B/LEGACYSECRET", before.Secret, "옮기기 전에도 읽힌다")
		assert.Equal(t, "slack", before.Name, "이름이 없던 행은 채널 종류를 이름으로 받는다")

		moved, err := channels.MigrateLegacySecrets(ctx)
		require.NoError(t, err)
		assert.Equal(t, 2, moved, "이 행과 데모 시드(000021)의 Slack 웹훅")

		var plaintextLeft int
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT count(*) FROM notification_configs WHERE config ?| ARRAY['webhook_url','bot_token','token','password','smtp_password']`).
			Scan(&plaintextLeft))
		assert.Zero(t, plaintextLeft, "평문 비밀이 남지 않는다")

		var config string
		require.NoError(t, pool.QueryRow(ctx, `SELECT config::text FROM notification_configs WHERE id = $1`, id).Scan(&config))
		assert.NotContains(t, config, "LEGACYSECRET")
		after, err := channels.Get(ctx, "org-legacy", id)
		require.NoError(t, err)
		assert.Equal(t, "https://hooks.slack.com/services/T/B/LEGACYSECRET", after.Secret)

		again, err := channels.MigrateLegacySecrets(ctx)
		require.NoError(t, err)
		assert.Equal(t, 0, again)
	})

	t.Run("비밀을 풀지 못하는 행은 그 행만 표시하고 목록은 돌아간다", func(t *testing.T) {
		good := &domain.Channel{OrgID: "org-broken", Name: "정상", Type: domain.ChannelSlack,
			Secret: "https://hooks.slack.com/services/T/B/GOOD", Events: []sharednotify.EventType{sharednotify.EventBackupFailed}, Active: true}
		require.NoError(t, channels.Create(ctx, good))
		var brokenID string
		require.NoError(t, pool.QueryRow(ctx, `
			INSERT INTO notification_configs (org_id, name, channel, config, secret_encrypted, events)
			VALUES ('org-broken', '키가 바뀐 채널', 'telegram', '{"chat_id": 12345}', 'bm90LWEtdmFsaWQtY2lwaGVydGV4dA==', ARRAY['backup_failed'])
			RETURNING id::text`).Scan(&brokenID))

		list, err := channels.List(ctx, "org-broken")
		require.NoError(t, err, "깨진 행 하나가 목록 전체를 막지 않는다")
		require.Len(t, list, 2)
		byID := map[string]*domain.Channel{}
		for _, ch := range list {
			byID[ch.ID] = ch
		}
		assert.False(t, byID[good.ID].SecretUnreadable)
		assert.Equal(t, "https://hooks.slack.com/services/T/B/GOOD", byID[good.ID].Secret)
		assert.True(t, byID[brokenID].SecretUnreadable)
		assert.Empty(t, byID[brokenID].Secret)
		assert.Equal(t, domain.Settings{}, byID[brokenID].Settings, "모양이 다른 설정(숫자 chat_id)은 비운다")

		assert.NoError(t, channels.Delete(ctx, "org-broken", brokenID), "깨진 채널도 지울 수 있다")
	})

	t.Run("키가 없으면 비밀을 저장하지 않는다", func(t *testing.T) {
		noKey := NewPostgresChannelRepository(pool, nil)
		err := noKey.Create(ctx, &domain.Channel{OrgID: "org-it", Name: "x", Type: domain.ChannelTelegram, Secret: itToken})
		assert.ErrorIs(t, err, ErrEncryptionKeyMissing)
	})
}

func setupPostgres(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	container, err := postgres.Run(ctx, "postgres:18",
		postgres.WithDatabase("nullus"), postgres.WithUsername("nullus"), postgres.WithPassword("nullus_dev"),
		testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").WithOccurrence(2)),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })
	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, runMigrations(ctx, pool))
	return pool
}

func runMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	_, thisFile, _, _ := runtime.Caller(0)
	dir := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..", "db", "migrations"))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var ups []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".up.sql") {
			ups = append(ups, e.Name())
		}
	}
	sort.Strings(ups)
	for _, name := range ups {
		sql, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("apply %s: %w", name, err)
		}
	}
	return nil
}
