package tokens

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func newTestManager(t *testing.T, accessTTL time.Duration) *Manager {
	t.Helper()
	// Empty key file paths => ephemeral in-process RSA key (local mode).
	m, err := NewManager("", "", accessTTL, time.Hour)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	return m
}

func TestAccessTokenRoundTrip(t *testing.T) {
	m := newTestManager(t, time.Hour)
	id := uuid.New()

	tok, err := m.IssueAccess(id)
	if err != nil {
		t.Fatalf("IssueAccess: %v", err)
	}
	got, err := m.VerifyAccess(tok)
	if err != nil {
		t.Fatalf("VerifyAccess: %v", err)
	}
	if got != id {
		t.Fatalf("subject mismatch: got %s want %s", got, id)
	}
}

func TestRefreshTokenRoundTrip(t *testing.T) {
	m := newTestManager(t, time.Hour)
	id := uuid.New()

	tok, err := m.IssueRefresh(id)
	if err != nil {
		t.Fatalf("IssueRefresh: %v", err)
	}
	if _, err := m.VerifyRefresh(tok); err != nil {
		t.Fatalf("VerifyRefresh: %v", err)
	}
}

func TestAccessTokenRejectedAsRefresh(t *testing.T) {
	m := newTestManager(t, time.Hour)
	id := uuid.New()

	access, _ := m.IssueAccess(id)
	if _, err := m.VerifyRefresh(access); err == nil {
		t.Fatal("an access token must not verify as a refresh token")
	}

	refresh, _ := m.IssueRefresh(id)
	if _, err := m.VerifyAccess(refresh); err == nil {
		t.Fatal("a refresh token must not verify as an access token")
	}
}

func TestExpiredAccessTokenRejected(t *testing.T) {
	m := newTestManager(t, -time.Minute) // already expired
	id := uuid.New()

	tok, _ := m.IssueAccess(id)
	if _, err := m.VerifyAccess(tok); err == nil {
		t.Fatal("expired token should not verify")
	}
}

func TestTamperedTokenRejected(t *testing.T) {
	m := newTestManager(t, time.Hour)
	tok, _ := m.IssueAccess(uuid.New())

	// Flip a character in the signature segment.
	b := []byte(tok)
	b[len(b)-1] ^= 0x01
	if _, err := m.VerifyAccess(string(b)); err == nil {
		t.Fatal("tampered token should not verify")
	}
}

func TestForeignKeyTokenRejected(t *testing.T) {
	m1 := newTestManager(t, time.Hour)
	m2 := newTestManager(t, time.Hour) // different ephemeral key
	tok, _ := m1.IssueAccess(uuid.New())
	if _, err := m2.VerifyAccess(tok); err == nil {
		t.Fatal("token signed by a different key must not verify")
	}
}
