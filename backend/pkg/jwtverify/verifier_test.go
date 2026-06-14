package jwtverify

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func writePubKey(t *testing.T, pub *rsa.PublicKey) string {
	t.Helper()
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	p := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})
	path := filepath.Join(t.TempDir(), "pub.pem")
	if err := os.WriteFile(path, p, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func sign(t *testing.T, key *rsa.PrivateKey, sub, tokenType string, exp time.Time) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss":        "gold-auth",
		"sub":        sub,
		"token_type": tokenType,
		"exp":        exp.Unix(),
	})
	s, err := tok.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestNewRequiresKeyOutsideLocal(t *testing.T) {
	if _, err := New("", "prod"); err == nil {
		t.Fatal("empty key file outside local must error (no fail-open)")
	}
	if _, err := New("", "local"); err != nil {
		t.Fatalf("empty key file in local should be permitted: %v", err)
	}
}

func TestVerifyAccessHappyPath(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	v, err := New(writePubKey(t, &key.PublicKey), "prod")
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.New()
	tok := sign(t, key, id.String(), "access", time.Now().Add(time.Hour))
	got, err := v.VerifyAccess(tok)
	if err != nil {
		t.Fatalf("VerifyAccess: %v", err)
	}
	if got != id {
		t.Fatalf("sub mismatch: %s != %s", got, id)
	}
}

func TestVerifyAccessRejectsRefreshType(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	v, _ := New(writePubKey(t, &key.PublicKey), "prod")
	tok := sign(t, key, uuid.NewString(), "refresh", time.Now().Add(time.Hour))
	if _, err := v.VerifyAccess(tok); err == nil {
		t.Fatal("a refresh token must not verify as access")
	}
}

func TestVerifyAccessRejectsExpiredAndForeignKey(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	v, _ := New(writePubKey(t, &key.PublicKey), "prod")

	expired := sign(t, key, uuid.NewString(), "access", time.Now().Add(-time.Minute))
	if _, err := v.VerifyAccess(expired); err == nil {
		t.Fatal("expired token must not verify")
	}

	other, _ := rsa.GenerateKey(rand.Reader, 2048)
	foreign := sign(t, other, uuid.NewString(), "access", time.Now().Add(time.Hour))
	if _, err := v.VerifyAccess(foreign); err == nil {
		t.Fatal("token signed by a different key must not verify")
	}
}

func TestPermissiveModeDecodesWithoutVerifying(t *testing.T) {
	v, _ := New("", "local")
	if !v.Permissive() {
		t.Fatal("expected permissive mode")
	}
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	id := uuid.New()
	tok := sign(t, key, id.String(), "access", time.Now().Add(time.Hour))
	got, err := v.VerifyAccess(tok) // signature not checked in permissive mode
	if err != nil {
		t.Fatalf("permissive verify: %v", err)
	}
	if got != id {
		t.Fatalf("sub mismatch: %s != %s", got, id)
	}
}
