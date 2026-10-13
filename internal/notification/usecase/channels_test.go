package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/notification/domain"
	sharednotify "github.com/cloud-nullus/draft/internal/shared/notification"
)

const tgToken = "123456789:AAHdqTcvCH1vGWJxfSeofSAs0K5PALDsaw0"

func tgInput(org string) CreateChannelInput {
	return CreateChannelInput{
		OrgID: org, Name: "텔레그램", Type: domain.ChannelTelegram,
		Settings: domain.Settings{ChatID: "-100123"}, Secret: tgToken,
		Events: []sharednotify.EventType{sharednotify.EventBackupFailed},
	}
}

func TestChannelService_Create_ValidatesAndDefaultsActive(t *testing.T) {
	repo := newFakeChannelRepo()
	svc := NewChannelService(repo)

	ch, err := svc.Create(context.Background(), tgInput("org-1"))
	require.NoError(t, err)
	assert.NotEmpty(t, ch.ID)
	assert.True(t, ch.Active, "새 채널은 켜진 채로 만든다")

	bad := tgInput("org-1")
	bad.Secret = ""
	_, err = svc.Create(context.Background(), bad)
	var ve *domain.ValidationError
	require.True(t, errors.As(err, &ve))
	assert.Equal(t, "secret", ve.Field)

	missingOrg := tgInput("")
	_, err = svc.Create(context.Background(), missingOrg)
	assert.Error(t, err)
}

func TestChannelService_OtherOrgCannotSeeOrChange(t *testing.T) {
	repo := newFakeChannelRepo()
	svc := NewChannelService(repo)
	ch, err := svc.Create(context.Background(), tgInput("org-1"))
	require.NoError(t, err)

	_, err = svc.Get(context.Background(), "org-2", ch.ID)
	assert.ErrorIs(t, err, domain.ErrChannelNotFound)
	name := "탈취"
	_, err = svc.Update(context.Background(), "org-2", ch.ID, domain.ChannelUpdate{Name: &name})
	assert.ErrorIs(t, err, domain.ErrChannelNotFound)
	assert.ErrorIs(t, svc.Delete(context.Background(), "org-2", ch.ID), domain.ErrChannelNotFound)

	list, err := svc.List(context.Background(), "org-2")
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestChannelService_Update_KeepsSecretWhenOmitted(t *testing.T) {
	repo := newFakeChannelRepo()
	svc := NewChannelService(repo)
	ch, err := svc.Create(context.Background(), tgInput("org-1"))
	require.NoError(t, err)

	events := []sharednotify.EventType{sharednotify.EventBackupFailed, sharednotify.EventBackupSucceeded}
	updated, err := svc.Update(context.Background(), "org-1", ch.ID, domain.ChannelUpdate{Events: &events})
	require.NoError(t, err)
	assert.Equal(t, events, updated.Events)

	stored, err := repo.Get(context.Background(), "org-1", ch.ID)
	require.NoError(t, err)
	assert.Equal(t, tgToken, stored.Secret)
}
