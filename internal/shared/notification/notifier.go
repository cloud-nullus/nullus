package notification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/smtp"
	"strings"
)

type Notifier interface {
	Send(ctx context.Context, msg Message) error
}

type Message struct {
	Channel   string
	Recipient string
	Subject   string
	Body      string
	Metadata  map[string]any
}

type SlackNotifier struct {
	webhookURL string
	httpClient *http.Client
}

type EmailNotifier struct {
	smtpHost string
	smtpPort int
	from     string
	password string
}

type MultiNotifier struct {
	notifiers map[string]Notifier
}

func (n *SlackNotifier) Send(ctx context.Context, msg Message) error {
	webhookURL := msg.Recipient
	if webhookURL == "" {
		webhookURL = n.webhookURL
	}
	if webhookURL == "" {
		return errors.New("slack webhook url is required")
	}

	httpClient := n.httpClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	payload := map[string]any{
		"channel":  msg.Channel,
		"subject":  msg.Subject,
		"body":     msg.Body,
		"metadata": msg.Metadata,
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal slack payload: %w", err)
	}

	bodyReader := strings.NewReader(string(payloadJSON))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bodyReader)
	if err != nil {
		return fmt.Errorf("build slack request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send slack request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		raw, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return fmt.Errorf("slack webhook failed: status=%d read body: %w", resp.StatusCode, readErr)
		}
		return fmt.Errorf("slack webhook failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	return nil
}

func (n *EmailNotifier) Send(ctx context.Context, msg Message) error {
	_ = ctx

	recipient := strings.TrimSpace(msg.Recipient)
	if recipient == "" {
		return errors.New("email recipient is required")
	}
	if n.smtpHost == "" {
		return errors.New("smtp host is required")
	}
	if n.smtpPort <= 0 {
		return errors.New("smtp port must be greater than zero")
	}
	if n.from == "" {
		return errors.New("smtp from address is required")
	}

	smtpAddr := fmt.Sprintf("%s:%d", n.smtpHost, n.smtpPort)
	auth := smtp.PlainAuth("", n.from, n.password, n.smtpHost)

	body := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s", n.from, recipient, msg.Subject, msg.Body)

	if err := smtp.SendMail(smtpAddr, auth, n.from, []string{recipient}, []byte(body)); err != nil {
		return fmt.Errorf("send email: %w", err)
	}

	return nil
}

func (n *MultiNotifier) Send(ctx context.Context, msg Message) error {
	if n == nil {
		return errors.New("multi notifier is nil")
	}
	notifier, ok := n.notifiers[msg.Channel]
	if !ok {
		return fmt.Errorf("unsupported notification channel: %s", msg.Channel)
	}
	if notifier == nil {
		return fmt.Errorf("notifier for channel %s is nil", msg.Channel)
	}

	if err := notifier.Send(ctx, msg); err != nil {
		return fmt.Errorf("send %s notification: %w", msg.Channel, err)
	}

	return nil
}
