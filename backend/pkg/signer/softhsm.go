//go:build cgo

package signer

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/asn1"
	"fmt"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/miekg/pkcs11"
)

// SoftHSMSigner implements Signer via PKCS#11.
// It has been tested against SoftHSM2 but works with any PKCS#11 v2.40+
// library that supports CKM_ECDSA on a secp256k1 key.
//
// Concurrency: SoftHSMSigner serialises C_Sign calls with a mutex because
// PKCS#11 sessions are not thread-safe for signing operations.
type SoftHSMSigner struct {
	mu         sync.Mutex
	ctx        *pkcs11.Ctx
	session    pkcs11.SessionHandle
	privHandle pkcs11.ObjectHandle
	pub        *ecdsa.PublicKey
	addr       [20]byte
}

// SoftHSMConfig holds the parameters needed to locate a key in a PKCS#11 token.
type SoftHSMConfig struct {
	// LibPath is the filesystem path to the PKCS#11 shared library.
	// e.g. "/usr/lib/softhsm/libsofthsm2.so" or "/usr/local/lib/softhsm/libsofthsm2.so"
	LibPath string
	// TokenLabel is the CKA_LABEL of the token slot to use.
	TokenLabel string
	// PIN is the user PIN for the token.
	PIN string
	// KeyLabel is the CKA_LABEL of the private-key object to use for signing.
	KeyLabel string
}

// NewSoftHSMSigner opens a PKCS#11 session against the named token, locates
// the private key by label, and derives the corresponding Ethereum address.
// Call Close() when the signer is no longer needed.
func NewSoftHSMSigner(cfg SoftHSMConfig) (*SoftHSMSigner, error) {
	p := pkcs11.New(cfg.LibPath)
	if err := p.Initialize(); err != nil {
		return nil, fmt.Errorf("softhsm signer: initialize PKCS#11 lib %q: %w", cfg.LibPath, err)
	}

	// Find the slot that hosts the requested token.
	slotID, err := findSlot(p, cfg.TokenLabel)
	if err != nil {
		p.Destroy()
		return nil, err
	}

	// Open a read-write session for signing.
	session, err := p.OpenSession(slotID, pkcs11.CKF_SERIAL_SESSION|pkcs11.CKF_RW_SESSION)
	if err != nil {
		p.Destroy()
		return nil, fmt.Errorf("softhsm signer: open session on slot %d: %w", slotID, err)
	}

	if err := p.Login(session, pkcs11.CKU_USER, cfg.PIN); err != nil {
		_ = p.CloseSession(session)
		p.Destroy()
		return nil, fmt.Errorf("softhsm signer: login: %w", err)
	}

	privHandle, err := findPrivKey(p, session, cfg.KeyLabel)
	if err != nil {
		_ = p.CloseSession(session)
		p.Destroy()
		return nil, err
	}

	pubKey, err := derivePubKey(p, session, cfg.KeyLabel)
	if err != nil {
		_ = p.CloseSession(session)
		p.Destroy()
		return nil, err
	}

	ethAddr := crypto.PubkeyToAddress(*pubKey)
	return &SoftHSMSigner{
		ctx:        p,
		session:    session,
		privHandle: privHandle,
		pub:        pubKey,
		addr:       [20]byte(ethAddr),
	}, nil
}

// Sign calls C_Sign with CKM_ECDSA mechanism, producing an IEEE P1363
// {r||s} signature (64 bytes), then appends the recovery byte v to form
// the 65-byte Ethereum-compact signature.
func (s *SoftHSMSigner) Sign(_ context.Context, digest [32]byte) ([65]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	mech := []*pkcs11.Mechanism{pkcs11.NewMechanism(pkcs11.CKM_ECDSA, nil)}
	if err := s.ctx.SignInit(s.session, mech, s.privHandle); err != nil {
		return [65]byte{}, fmt.Errorf("softhsm signer: SignInit: %w", err)
	}

	// PKCS#11 CKM_ECDSA returns the raw IEEE P1363 encoding: {r[32] || s[32]}
	// for a 256-bit key.
	raw, err := s.ctx.Sign(s.session, digest[:])
	if err != nil {
		return [65]byte{}, fmt.Errorf("softhsm signer: Sign: %w", err)
	}

	if len(raw) != 64 {
		return [65]byte{}, fmt.Errorf("softhsm signer: unexpected signature length %d (expected 64)", len(raw))
	}

	r := new(big.Int).SetBytes(raw[:32])
	bigS := new(big.Int).SetBytes(raw[32:])

	// Ethereum requires a low-s value (EIP-2).
	secp256k1N := crypto.S256().Params().N
	halfN := new(big.Int).Rsh(secp256k1N, 1)
	if bigS.Cmp(halfN) > 0 {
		bigS.Sub(secp256k1N, bigS)
	}

	// Determine the recovery byte v by trying 0 and 1.
	var sig65 [65]byte
	copy(sig65[:32], r.Bytes())
	copy(sig65[32:64], bigS.Bytes())

	for v := byte(0); v <= 1; v++ {
		sig65[64] = v
		recovered, err := crypto.SigToPub(digest[:], sig65[:])
		if err != nil {
			continue
		}
		if recovered.X.Cmp(s.pub.X) == 0 && recovered.Y.Cmp(s.pub.Y) == 0 {
			return sig65, nil
		}
	}

	return [65]byte{}, fmt.Errorf("softhsm signer: could not recover public key from signature")
}

// PublicKey returns the ECDSA public key.
func (s *SoftHSMSigner) PublicKey() *ecdsa.PublicKey { return s.pub }

// Address returns the Ethereum address derived from the public key.
func (s *SoftHSMSigner) Address() [20]byte { return s.addr }

// Close logs out, closes the PKCS#11 session, and finalizes the library.
// After Close() the signer must not be used.
func (s *SoftHSMSigner) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.ctx.Logout(s.session)
	_ = s.ctx.CloseSession(s.session)
	s.ctx.Destroy()
	return nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

// findSlot returns the slot ID whose CKT_TOKEN_INFO label matches tokenLabel.
func findSlot(p *pkcs11.Ctx, tokenLabel string) (uint, error) {
	slots, err := p.GetSlotList(true)
	if err != nil {
		return 0, fmt.Errorf("softhsm signer: get slot list: %w", err)
	}
	for _, slot := range slots {
		info, err := p.GetTokenInfo(slot)
		if err != nil {
			continue
		}
		// Token labels are padded to 32 bytes; trim before comparing.
		if trimPad(info.Label) == tokenLabel {
			return slot, nil
		}
	}
	return 0, fmt.Errorf("softhsm signer: token with label %q not found", tokenLabel)
}

// findPrivKey returns the handle of the private key object with the given label.
func findPrivKey(p *pkcs11.Ctx, session pkcs11.SessionHandle, label string) (pkcs11.ObjectHandle, error) {
	template := []*pkcs11.Attribute{
		pkcs11.NewAttribute(pkcs11.CKA_CLASS, pkcs11.CKO_PRIVATE_KEY),
		pkcs11.NewAttribute(pkcs11.CKA_LABEL, label),
	}
	if err := p.FindObjectsInit(session, template); err != nil {
		return 0, fmt.Errorf("softhsm signer: FindObjectsInit (priv, %q): %w", label, err)
	}
	defer func() { _ = p.FindObjectsFinal(session) }()

	handles, _, err := p.FindObjects(session, 1)
	if err != nil {
		return 0, fmt.Errorf("softhsm signer: FindObjects (priv, %q): %w", label, err)
	}
	if len(handles) == 0 {
		return 0, fmt.Errorf("softhsm signer: private key with label %q not found", label)
	}
	return handles[0], nil
}

// derivePubKey reads the CKA_EC_POINT of the matching public key object and
// returns the decoded *ecdsa.PublicKey on the secp256k1 curve.
func derivePubKey(p *pkcs11.Ctx, session pkcs11.SessionHandle, label string) (*ecdsa.PublicKey, error) {
	template := []*pkcs11.Attribute{
		pkcs11.NewAttribute(pkcs11.CKA_CLASS, pkcs11.CKO_PUBLIC_KEY),
		pkcs11.NewAttribute(pkcs11.CKA_LABEL, label),
	}
	if err := p.FindObjectsInit(session, template); err != nil {
		return nil, fmt.Errorf("softhsm signer: FindObjectsInit (pub, %q): %w", label, err)
	}
	defer func() { _ = p.FindObjectsFinal(session) }()

	handles, _, err := p.FindObjects(session, 1)
	if err != nil {
		return nil, fmt.Errorf("softhsm signer: FindObjects (pub, %q): %w", label, err)
	}
	if len(handles) == 0 {
		return nil, fmt.Errorf("softhsm signer: public key with label %q not found", label)
	}

	attrs, err := p.GetAttributeValue(session, handles[0], []*pkcs11.Attribute{
		pkcs11.NewAttribute(pkcs11.CKA_EC_POINT, nil),
	})
	if err != nil {
		return nil, fmt.Errorf("softhsm signer: get CKA_EC_POINT: %w", err)
	}
	if len(attrs) == 0 || len(attrs[0].Value) == 0 {
		return nil, fmt.Errorf("softhsm signer: empty CKA_EC_POINT for key %q", label)
	}

	// CKA_EC_POINT is a DER-encoded octet string wrapping the uncompressed point.
	var pointBytes []byte
	if _, err := asn1.Unmarshal(attrs[0].Value, &pointBytes); err != nil {
		// Some tokens store the raw point bytes without the outer OCTET STRING wrapper.
		pointBytes = attrs[0].Value
	}

	// Expect uncompressed point: 0x04 || x[32] || y[32]
	if len(pointBytes) != 65 || pointBytes[0] != 0x04 {
		return nil, fmt.Errorf("softhsm signer: unexpected EC point format (len=%d, prefix=0x%02x)", len(pointBytes), pointBytes[0])
	}

	curve := crypto.S256() // secp256k1
	x := new(big.Int).SetBytes(pointBytes[1:33])
	y := new(big.Int).SetBytes(pointBytes[33:65])

	pub := &ecdsa.PublicKey{Curve: elliptic.Curve(curve), X: x, Y: y}
	return pub, nil
}

// trimPad removes trailing ASCII space padding from PKCS#11 fixed-length strings.
func trimPad(s string) string {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] != ' ' {
			return s[:i+1]
		}
	}
	return ""
}
