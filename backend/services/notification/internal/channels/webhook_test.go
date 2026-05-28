package channels

import (
	"net"
	"net/url"
	"testing"
)

func TestIsBlockedIP(t *testing.T) {
	blocked := []string{
		"127.0.0.1",       // loopback
		"10.1.2.3",        // private
		"192.168.0.5",     // private
		"172.16.9.9",      // private
		"169.254.169.254", // link-local (cloud metadata)
		"0.0.0.0",         // unspecified
		"::1",             // IPv6 loopback
		"fd00::1",         // IPv6 ULA (private)
	}
	for _, s := range blocked {
		if !isBlockedIP(net.ParseIP(s)) {
			t.Errorf("expected %s to be blocked", s)
		}
	}
	allowed := []string{"8.8.8.8", "1.1.1.1", "93.184.216.34"}
	for _, s := range allowed {
		if isBlockedIP(net.ParseIP(s)) {
			t.Errorf("expected %s to be allowed", s)
		}
	}
}

func TestValidateWebhookURL(t *testing.T) {
	bad := []string{
		"file:///etc/passwd",
		"ftp://example.com/x",
		"http://169.254.169.254/latest/meta-data/",
		"https://127.0.0.1/secret",
		"http://10.0.0.1:8089/internal",
		"https://",
	}
	for _, raw := range bad {
		u, err := url.Parse(raw)
		if err != nil {
			continue // unparseable is also fine (rejected upstream)
		}
		if err := validateWebhookURL(u); err == nil {
			t.Errorf("expected %q to be rejected", raw)
		}
	}

	good := []string{"https://hooks.example.com/abc", "http://example.org/cb"}
	for _, raw := range good {
		u, _ := url.Parse(raw)
		if err := validateWebhookURL(u); err != nil {
			t.Errorf("expected %q to be allowed, got %v", raw, err)
		}
	}
}
