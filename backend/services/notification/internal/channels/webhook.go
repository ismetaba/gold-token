package channels

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"
)

const (
	webhookMaxRetries = 3
	webhookBaseDelay  = 500 * time.Millisecond
)

// isBlockedIP reports whether an address must not be reachable by a
// user-configured webhook, to prevent SSRF into internal infrastructure
// (including the cloud metadata endpoint at 169.254.169.254, which is
// link-local).
func isBlockedIP(ip net.IP) bool {
	return ip == nil ||
		ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsMulticast()
}

// validateWebhookURL rejects non-http(s) schemes and IP-literal hosts that
// point at internal ranges. DNS hostnames are additionally checked at dial
// time (see guardedDialContext), which also defeats DNS-rebinding.
func validateWebhookURL(u *url.URL) error {
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("webhook scheme %q not allowed (use http or https)", u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("webhook host is empty")
	}
	if ip := net.ParseIP(host); ip != nil && isBlockedIP(ip) {
		return fmt.Errorf("webhook host %s is not an allowed address", ip)
	}
	return nil
}

// guardedDialContext resolves the target host and refuses to connect to any
// blocked address, then dials a vetted IP directly so the connection cannot be
// re-pointed at an internal host between resolution and dialing.
func guardedDialContext() func(ctx context.Context, network, addr string) (net.Conn, error) {
	d := &net.Dialer{Timeout: 5 * time.Second}
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, err
		}
		for _, ip := range ips {
			if isBlockedIP(ip.IP) {
				return nil, fmt.Errorf("refusing to connect to blocked address %s (SSRF protection)", ip.IP)
			}
		}
		return d.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
	}
}

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

// NewWebhookSender constructs a WebhookSender whose HTTP client blocks
// connections to internal/private addresses and re-validates redirect targets.
func NewWebhookSender() *WebhookSender {
	return &WebhookSender{
		client: &http.Client{
			Timeout:   10 * time.Second,
			Transport: &http.Transport{DialContext: guardedDialContext()},
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return fmt.Errorf("stopped after 5 redirects")
				}
				return validateWebhookURL(req.URL)
			},
		},
	}
}

// Send POSTs payload to targetURL, retrying on transient failures.
func (s *WebhookSender) Send(ctx context.Context, targetURL, eventType, subject, body string) error {
	if targetURL == "" {
		return fmt.Errorf("webhook_url is empty")
	}
	u, err := url.Parse(targetURL)
	if err != nil {
		return fmt.Errorf("invalid webhook url: %w", err)
	}
	if err := validateWebhookURL(u); err != nil {
		return err
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
