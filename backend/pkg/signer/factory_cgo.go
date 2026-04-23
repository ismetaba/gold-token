//go:build cgo

package signer

import "fmt"

func newSoftHSMSigner(cfg Config) (Signer, error) {
	hsmCfg := SoftHSMConfig{
		LibPath:    cfg.LibPath,
		TokenLabel: cfg.TokenLabel,
		PIN:        cfg.PIN,
		KeyLabel:   cfg.KeyLabel,
	}
	if hsmCfg.LibPath == "" {
		return nil, fmt.Errorf("signer: SOFTHSM2_LIB must be set for softhsm signer")
	}
	if hsmCfg.TokenLabel == "" {
		return nil, fmt.Errorf("signer: SOFTHSM2_TOKEN_LABEL must be set for softhsm signer")
	}
	if hsmCfg.KeyLabel == "" {
		return nil, fmt.Errorf("signer: SOFTHSM2_KEY_LABEL must be set for softhsm signer")
	}
	return NewSoftHSMSigner(hsmCfg)
}
