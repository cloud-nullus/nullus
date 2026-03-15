package notification

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSlackNotifier_Send_SendsJSONPayload(t *testing.T) {
	t.Parallel()

	type slackPayload struct {
		Channel  string         `json:"channel"`
		Subject  string         `json:"subject"`
		Body     string         `json:"body"`
		Metadata map[string]any `json:"metadata"`
	}

	received := slackPayload{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&received))
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	notifier := &SlackNotifier{
		webhookURL: server.URL,
		httpClient: server.Client(),
	}

	err := notifier.Send(context.Background(), Message{
		Channel:   "slack",
		Recipient: server.URL,
		Subject:   "Deploy failed",
		Body:      "pipeline api-release is red",
		Metadata: map[string]any{
			"org_id": "org-1",
		},
	})
	require.NoError(t, err)

	assert.Equal(t, "slack", received.Channel)
	assert.Equal(t, "Deploy failed", received.Subject)
	assert.Equal(t, "pipeline api-release is red", received.Body)
	assert.Equal(t, "org-1", received.Metadata["org_id"])
}

type fakeNotifier struct {
	sendFn func(ctx context.Context, msg Message) error
}

func (f *fakeNotifier) Send(ctx context.Context, msg Message) error {
	if f.sendFn == nil {
		return nil
	}
	return f.sendFn(ctx, msg)
}

func TestMultiNotifier_Send_RoutesByChannel(t *testing.T) {
	t.Parallel()

	called := 0
	n := &MultiNotifier{
		notifiers: map[string]Notifier{
			"slack": &fakeNotifier{sendFn: func(_ context.Context, msg Message) error {
				called++
				assert.Equal(t, "slack", msg.Channel)
				return nil
			}},
		},
	}

	err := n.Send(context.Background(), Message{Channel: "slack", Body: "ok"})
	require.NoError(t, err)
	assert.Equal(t, 1, called)
}
