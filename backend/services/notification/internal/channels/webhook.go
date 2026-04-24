package channels

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const (
	webhookMaxRetries = 3
	webhookBaseDelay  = 500 * time.Millisecond
)

// WebhookPayload is the JSON body posted to the user's webhook URL.
type WebhookPayload struct {
	EventType string `json:"event_type"`
	Subject   string `json:"subject"`
	Body      string `json:"body"`
	SentAt    string `json:"sent_at"`
}

// WebhookSender delivers notifications via HTTP POST to a user-supplied URL.
// Failed attempts are retried up to 3 times with exponential backoff.
type WebhookSender struct {
	client *http.Client
}

// NewWebhookSender constructs a WebhookSender.
func NewWebhookSender() *WebhookSender {
	return &WebhookSender{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Send POSTs payload to targetURL, retrying on transient failures.
func (s *WebhookSender) Send(ctx context.Context, targetURL, eventType, subject, body string) error {
	if targetURL == "" {
		return fmt.Errorf("webhook_url is empty")
	}

	p := WebhookPayload{
		EventType: eventType,
		Subject:   subject,
		Body:      body,
		SentAt:    time.Now().UTC().Format(time.RFC3339),
	}
	b, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("marshal webhook payload: %w", err)
	}

	var lastErr error
	delay := webhookBaseDelay
	for attempt := 0; attempt < webhookMaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
				delay *= 2
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(b))
		if err != nil {
			return fmt.Errorf("build webhook request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Gold-Event", eventType)

		resp, err := s.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("webhook attempt %d: %w", attempt+1, err)
			continue
		}
		_ = resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}

		// Treat 4xx as permanent failure — no point retrying.
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return fmt.Errorf("webhook returned %d (client error, not retrying)", resp.StatusCode)
		}

		lastErr = fmt.Errorf("webhook attempt %d: status %d", attempt+1, resp.StatusCode)
	}

	return fmt.Errorf("webhook failed after %d attempts: %w", webhookMaxRetries, lastErr)
}
