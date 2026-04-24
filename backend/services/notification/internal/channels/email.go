// Package channels provides delivery channel implementations for the notification service.
package channels

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"time"
)

// EmailSender delivers notifications via SMTP or SendGrid.
// In local mode (no SMTP_HOST and no SENDGRID_API_KEY) it logs and returns nil.
type EmailSender struct {
	smtpHost       string
	smtpPort       string
	smtpUser       string
	smtpPass       string
	smtpFrom       string
	sendGridAPIKey string
	client         *http.Client
}

// NewEmailSender constructs an EmailSender. Any combination of SMTP / SendGrid
// config may be provided; SMTP is preferred when smtpHost is set.
func NewEmailSender(smtpHost, smtpPort, smtpUser, smtpPass, smtpFrom, sendGridAPIKey string) *EmailSender {
	return &EmailSender{
		smtpHost:       smtpHost,
		smtpPort:       smtpPort,
		smtpUser:       smtpUser,
		smtpPass:       smtpPass,
		smtpFrom:       smtpFrom,
		sendGridAPIKey: sendGridAPIKey,
		client:         &http.Client{Timeout: 15 * time.Second},
	}
}

// Send delivers an email to the given address with the given subject and body.
func (s *EmailSender) Send(_ context.Context, to, subject, body string) error {
	if to == "" {
		return nil // no recipient — silently skip
	}
	switch {
	case s.smtpHost != "":
		return s.sendSMTP(to, subject, body)
	case s.sendGridAPIKey != "":
		return s.sendSendGrid(to, subject, body)
	default:
		// Local stub — pretend we sent it.
		return nil
	}
}

func (s *EmailSender) sendSMTP(to, subject, body string) error {
	from := s.smtpFrom
	if from == "" {
		from = "noreply@gold-token.io"
	}

	msg := []byte(fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		from, to, subject, body,
	))

	addr := fmt.Sprintf("%s:%s", s.smtpHost, s.smtpPort)

	var auth smtp.Auth
	if s.smtpUser != "" {
		auth = smtp.PlainAuth("", s.smtpUser, s.smtpPass, s.smtpHost)
	}

	if err := smtp.SendMail(addr, auth, from, []string{to}, msg); err != nil {
		return fmt.Errorf("smtp send: %w", err)
	}
	return nil
}

func (s *EmailSender) sendSendGrid(to, subject, body string) error {
	from := s.smtpFrom
	if from == "" {
		from = "noreply@gold-token.io"
	}

	payload := map[string]any{
		"personalizations": []map[string]any{
			{"to": []map[string]string{{"email": to}}},
		},
		"from":    map[string]string{"email": from},
		"subject": subject,
		"content": []map[string]string{
			{"type": "text/plain", "value": body},
		},
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal sendgrid payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, "https://api.sendgrid.com/v3/mail/send", bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("build sendgrid request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.sendGridAPIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("sendgrid request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("sendgrid returned %d", resp.StatusCode)
	}
	return nil
}
